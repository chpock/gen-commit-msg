package server

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os/exec"
	"regexp"
	"strings"
	"syscall"
	"time"
)

var listenURLRe = regexp.MustCompile(`opencode server listening on (http://[^\s]+)`)

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
	cmdCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	s.cancel = cancel

	s.cmd = exec.CommandContext(cmdCtx, "opencode", "serve", "--hostname", "127.0.0.1", "--port", "0")
	s.cmd.SysProcAttr = &syscall.SysProcAttr{
		Pdeathsig: syscall.SIGKILL,
		Setpgid:   true,
	}

	stdout, err := s.cmd.StdoutPipe()
	if err != nil {
		cancel()
		return "", fmt.Errorf("stdout pipe: %w", err)
	}
	s.cmd.Stderr = nil // discard opencode stderr

	if err := s.cmd.Start(); err != nil {
		cancel()
		return "", fmt.Errorf("start opencode: %w", err)
	}

	baseURL, err := parseListenURL(stdout, 30*time.Second)
	if err != nil {
		s.Stop()
		return "", fmt.Errorf("parse listen URL: %w", err)
	}
	s.baseURL = baseURL

	if err := healthCheck(ctx, baseURL); err != nil {
		s.Stop()
		return "", fmt.Errorf("health check: %w", err)
	}

	return baseURL, nil
}

func parseListenURL(r io.Reader, timeout time.Duration) (string, error) {
	ch := make(chan string, 1)
	errCh := make(chan error, 1)

	go func() {
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			line := scanner.Text()
			if matches := listenURLRe.FindStringSubmatch(line); len(matches) > 1 {
				ch <- matches[1]
				return
			}
		}
		errCh <- errors.New("opencode exited without printing listen URL")
	}()

	select {
	case url := <-ch:
		return url, nil
	case err := <-errCh:
		return "", err
	case <-time.After(timeout):
		return "", errors.New("timed out waiting for opencode listen URL")
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
		// If /health returns an error, try connecting to the host:port to check if it's listening
		return checkListen(baseURL)
	}
	defer resp.Body.Close()
	return nil
}

func checkListen(baseURL string) error {
	addr := strings.TrimPrefix(baseURL, "http://")
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		addr = baseURL
	}
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), 3*time.Second)
	if err != nil {
		return fmt.Errorf("server not listening on %s: %w", addr, err)
	}
	conn.Close()
	return nil
}

func (s *ProcessServer) Stop() error {
	if s.cancel != nil {
		s.cancel()
	}
	if s.cmd != nil && s.cmd.Process != nil {
		s.cmd.Process.Signal(syscall.SIGTERM)
		done := make(chan error, 1)
		go func() { done <- s.cmd.Wait() }()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			s.cmd.Process.Kill()
		}
	}
	return nil
}
