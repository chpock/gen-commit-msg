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
	cmd     *exec.Cmd
	cancel  context.CancelFunc
	baseURL string
}

func New() *ProcessServer {
	return &ProcessServer{}
}

func (s *ProcessServer) Start(ctx context.Context) (string, error) {
	// Use a background context for the server's lifetime, to be cancelled by Stop()
	serverCtx, serverCancel := context.WithCancel(context.Background())
	s.cancel = serverCancel // Store for Stop() to call

	// Use a separate context with a timeout for the startup phase (URL parsing and health check)
	startupCtx, startupCancel := context.WithTimeout(ctx, 30*time.Second)
	defer startupCancel() // Ensure this context is cancelled when Start() exits

	s.cmd = exec.CommandContext(serverCtx, "opencode", "serve", "--hostname", "127.0.0.1", "--port", "0")
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
		serverCancel()
		return "", fmt.Errorf("marshal opencode config: %w", err)
	}
	s.cmd.Env = append(os.Environ(), "OPENCODE_CONFIG_CONTENT="+string(cfgJSON))
	slog.Debug("setting OPENCODE_CONFIG_CONTENT for opencode serve process", "value", string(cfgJSON))

	slog.Info("starting opencode server", "args", s.cmd.Args[1:])

	stdout, err := s.cmd.StdoutPipe()
	if err != nil {
		slog.Error("failed to create stdout pipe", "error", err)
		serverCancel()
		return "", fmt.Errorf("stdout pipe: %w", err)
	}
	var stderrBuf bytes.Buffer
	s.cmd.Stderr = &stderrBuf

	if err := s.cmd.Start(); err != nil {
		slog.Error("failed to start opencode process", "error", err)
		serverCancel()
		if errors.Is(err, exec.ErrNotFound) {
			return "", fmt.Errorf("%w: %w", ErrOpenCodeNotFound, err)
		}
		return "", fmt.Errorf("start opencode: %w", err)
	}

	// Pass startupCtx to parseListenURL and healthCheck
	baseURL, err := parseListenURL(stdout, startupCtx)
	if err != nil {
		slog.Error("failed to parse listen URL", "error", err)
		_ = s.Stop()
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
		_ = s.Stop()
		return "", fmt.Errorf("health check: %w", err)
	}

	slog.Info("opencode server healthy", "url", baseURL)
	return baseURL, nil
}

func parseListenURL(r io.Reader, ctx context.Context) (string, error) {
	ch := make(chan string, 1)
	errCh := make(chan error, 1)

	go func() {
		var output strings.Builder
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			line := scanner.Text()
			output.WriteString(line)
			output.WriteByte('\n')
			if matches := listenURLRe.FindStringSubmatch(line); len(matches) > 1 {
				ch <- matches[1]
				return
			}
		}
		errCh <- fmt.Errorf("%w. Output:\n%s", ErrServerExited, output.String())
	}()

	select {
	case url := <-ch:
		return url, nil
	case err := <-errCh:
		return "", err
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
	slog.Info("stopping opencode server")
	if s.cancel != nil {
		s.cancel()
	}
	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Signal(syscall.SIGTERM)
		done := make(chan error, 1)
		go func() { done <- s.cmd.Wait() }()
		select {
		case <-done:
			slog.Info("opencode process exited cleanly")
		case <-time.After(5 * time.Second):
			slog.Warn("opencode server did not stop gracefully, killing")
			_ = s.cmd.Process.Kill()
		}
	}
	return nil
}
