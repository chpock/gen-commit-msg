package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"
)

type opencodeConfig struct {
	Snapshot   bool   `json:"snapshot"`
	Share      string `json:"share"`
	Autoupdate bool   `json:"autoupdate"`
}

var (
	listenURLRe = regexp.MustCompile(`opencode server listening on (http://[^\s]+)`)

	ErrOpenCodeNotFound = errors.New("opencode not found in PATH")
	ErrServerTimeout    = errors.New("timed out waiting for opencode listen URL")
	ErrServerExited     = errors.New("opencode exited without printing listen URL")
)

type Server interface {
	Start(ctx context.Context) (baseURL string, err error)
	Stop() error
}

type ProcessServer struct {
	executable string
	cmd        *exec.Cmd
	baseURL    string
	done       chan struct{}
	waitErr    error
	drainErr   error
	stopOnce   sync.Once
	stopErr    error
	stopGrace  time.Duration
}

func New(executable string) *ProcessServer {
	return &ProcessServer{executable: executable, stopGrace: 5 * time.Second}
}

func (s *ProcessServer) Start(ctx context.Context) (string, error) {
	// Use a separate context with a timeout for the startup phase (URL parsing and health check)
	startupCtx, startupCancel := context.WithTimeout(ctx, 30*time.Second)
	defer startupCancel() // Ensure this context is cancelled when Start() exits

	s.cmd = exec.Command(s.executable, "serve", "--hostname", "127.0.0.1", "--port", "0")
	s.cmd.SysProcAttr = &syscall.SysProcAttr{
		Pdeathsig: syscall.SIGKILL,
		Setpgid:   true,
	}

	cfg := opencodeConfig{
		// OpenCode's snapshot system (enabled by default) operates above git
		// and can modify the staging area when GIT_INDEX_FILE is set — which is
		// the case during commit creation. We disable it since gen-commit-msg
		// must not modify any files.
		// See: https://github.com/anomalyco/opencode/issues/9359
		//      https://github.com/anomalyco/opencode/issues/22477
		Snapshot:   false,
		Share:      "disabled",
		Autoupdate: false,
	}
	cfgJSON, err := json.Marshal(cfg)
	if err != nil {
		slog.Error("failed to marshal opencode config", "error", err)
		return "", fmt.Errorf("marshal opencode config: %w", err)
	}
	s.cmd.Env = append(os.Environ(), "OPENCODE_CONFIG_CONTENT="+string(cfgJSON))
	slog.Debug("setting OPENCODE_CONFIG_CONTENT for opencode serve process", "value", string(cfgJSON))

	slog.Info("starting opencode server", "args", s.cmd.Args[1:])

	// Own the pipe so Wait cannot close it before the output drain completes.
	stdout, stdoutWriter, err := os.Pipe()
	if err != nil {
		slog.Error("failed to create stdout pipe", "error", err)
		return "", fmt.Errorf("stdout pipe: %w", err)
	}
	s.cmd.Stdout = stdoutWriter
	var stderrBuf bytes.Buffer
	s.cmd.Stderr = &stderrBuf

	if err := s.cmd.Start(); err != nil {
		_ = stdout.Close()
		_ = stdoutWriter.Close()
		slog.Error("failed to start opencode process", "error", err)
		if errors.Is(err, exec.ErrNotFound) {
			return "", fmt.Errorf("%w: %w", ErrOpenCodeNotFound, err)
		}
		return "", fmt.Errorf("start opencode: %w", err)
	}

	_ = stdoutWriter.Close()
	listen := make(chan listenResult, 1)
	drained := make(chan struct{})
	go func() {
		defer close(drained)
		defer func() { _ = stdout.Close() }()
		s.drainErr = scanOutput(stdout, listen)
	}()
	s.done = make(chan struct{})
	go func() {
		s.waitErr = s.cmd.Wait()
		<-drained
		close(s.done)
	}()

	baseURL, err := parseListenURL(startupCtx, listen)
	if err != nil {
		slog.Error("failed to parse listen URL", "error", err)
		err = errors.Join(err, s.Stop())
		if stderrBuf.Len() > 0 {
			slog.Debug("opencode stderr", "output", stderrBuf.String())
			return "", fmt.Errorf("parse listen URL: %w\nstderr: %s", err, stderrBuf.String())
		}
		return "", fmt.Errorf("parse listen URL: %w", err)
	}
	slog.Debug("parsed listen URL", "url", baseURL)
	s.baseURL = baseURL

	if err := healthCheck(startupCtx, baseURL); err != nil {
		slog.Error("health check failed", "url", baseURL, "error", err)
		return "", fmt.Errorf("health check: %w", errors.Join(err, s.Stop()))
	}

	slog.Info("opencode server healthy", "url", baseURL)
	return baseURL, nil
}

type listenResult struct {
	url string
	err error
}

func scanOutput(r io.Reader, listen chan<- listenResult) error {
	var output strings.Builder
	found := false
	var err error
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		output.WriteString(line)
		output.WriteByte('\n')
		if matches := listenURLRe.FindStringSubmatch(line); len(matches) > 1 {
			listen <- listenResult{url: matches[1]}
			found = true
			_, err = io.Copy(io.Discard, r)
			break
		}
	}
	if !found {
		err = scanner.Err()
		listen <- listenResult{err: fmt.Errorf("%w. Output:\n%s", errors.Join(ErrServerExited, err), output.String())}
	}
	if err != nil {
		return fmt.Errorf("read opencode stdout: %w", err)
	}
	return nil
}

func parseListenURL(ctx context.Context, listen <-chan listenResult) (string, error) {
	select {
	case result := <-listen:
		return result.url, result.err
	case <-ctx.Done():
		slog.Debug("context done while waiting for listen URL", "err", ctx.Err())
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return "", ErrServerTimeout
		}
		return "", fmt.Errorf("%w: %w", ErrServerTimeout, ctx.Err())
	}
}

func healthCheck(ctx context.Context, baseURL string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/health", nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return checkListen(baseURL)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	return checkListen(baseURL)
}

func checkListen(baseURL string) error {
	addr := strings.TrimPrefix(baseURL, "http://")
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
		port = ""
	}
	var dialAddr string
	if port != "" {
		dialAddr = net.JoinHostPort(host, port)
	} else {
		dialAddr = host
	}
	conn, err := net.DialTimeout("tcp", dialAddr, 3*time.Second)
	if err != nil {
		return fmt.Errorf("server not listening on %s: %w", dialAddr, err)
	}
	_ = conn.Close()
	return nil
}

func (s *ProcessServer) Stop() error {
	if s.done == nil {
		return nil
	}
	s.stopOnce.Do(func() {
		s.stopErr = s.stop()
		if s.stopErr != nil {
			slog.Error("opencode server cleanup failed", "error", s.stopErr)
		} else {
			slog.Info("opencode process exited cleanly")
		}
	})
	return s.stopErr
}

func (s *ProcessServer) stop() error {
	slog.Info("stopping opencode server")
	select {
	case <-s.done:
		return errors.Join(errors.New("opencode exited before stop request"), s.waitErr, s.drainErr)
	default:
	}

	signalErr := s.cmd.Process.Signal(syscall.SIGTERM)
	requested := signalErr == nil
	if errors.Is(signalErr, os.ErrProcessDone) {
		signalErr = fmt.Errorf("opencode already exited when sending SIGTERM: %w", signalErr)
	} else if signalErr != nil {
		signalErr = fmt.Errorf("signal opencode with SIGTERM: %w", signalErr)
	}

	timer := time.NewTimer(s.stopGrace)
	defer timer.Stop()
	var forcedErr error
	select {
	case <-s.done:
	case <-timer.C:
		// Exit may race the deadline; do not claim a kill unless it is needed.
		select {
		case <-s.done:
		default:
			forcedErr = fmt.Errorf("opencode did not stop within %s; forced termination required", s.stopGrace)
			slog.Warn("opencode server shutdown incomplete, killing process group")
			// Descendants can retain output pipes after the direct server exits.
			if err := syscall.Kill(-s.cmd.Process.Pid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
				forcedErr = errors.Join(forcedErr, fmt.Errorf("kill opencode process group: %w", err))
			}
			<-s.done
		}
	}

	waitErr := s.waitErr
	var exitErr *exec.ExitError
	if requested && errors.As(waitErr, &exitErr) {
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.Signaled() && status.Signal() == syscall.SIGTERM {
			// Compatible with the requested signal, not proof of which sender caused exit.
			waitErr = nil
		}
	}
	if waitErr != nil {
		waitErr = fmt.Errorf("wait for opencode after stop request: %w", waitErr)
	}
	return errors.Join(signalErr, forcedErr, waitErr, s.drainErr)
}
