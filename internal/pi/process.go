package pi

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

type outputLine struct {
	data       []byte
	terminated bool
	err        error
}

type process struct {
	cmd             *exec.Cmd
	stdin           *os.File
	lines           chan outputLine
	discard         chan struct{}
	done            chan struct{}
	waitErr         error
	writers         sync.WaitGroup
	stderr          stderrTail
	operationFailed bool
}

func startProcess(executable, dir string, args ...string) (*process, error) {
	cmd := exec.Command(executable, args...)
	cmd.Dir = dir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	inR, inW, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	outR, outW, err := os.Pipe()
	if err != nil {
		_ = inR.Close()
		_ = inW.Close()
		return nil, err
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		_ = inR.Close()
		_ = inW.Close()
		_ = outR.Close()
		_ = outW.Close()
		return nil, err
	}
	cmd.Stdin = inR
	cmd.Stdout = outW
	cmd.Stderr = errW
	err = cmd.Start()
	_ = inR.Close()
	_ = outW.Close()
	_ = errW.Close()
	if err != nil {
		_ = inW.Close()
		_ = outR.Close()
		_ = errR.Close()
		return nil, err
	}
	p := &process{cmd: cmd, stdin: inW, lines: make(chan outputLine, 16), discard: make(chan struct{}), done: make(chan struct{})}
	slog.Info("Pi process started", "pid", cmd.Process.Pid, "version_preflight", len(args) == 1 && args[0] == "--version")
	var drains sync.WaitGroup
	drains.Add(2)
	// Own the pipes: exec.Wait must not close stdout before its buffered final records are read.
	go func() {
		defer drains.Done()
		defer func() { _ = outR.Close() }()
		defer close(p.lines)
		r := bufio.NewReader(outR)
		for {
			data, readErr := r.ReadBytes('\n')
			if len(data) > 0 {
				line := outputLine{data: data, terminated: data[len(data)-1] == '\n'}
				select {
				case p.lines <- line:
				case <-p.discard:
				}
			}
			if readErr != nil {
				if !errors.Is(readErr, io.EOF) {
					select {
					case p.lines <- outputLine{err: readErr}:
					case <-p.discard:
					}
				}
				return
			}
		}
	}()
	go func() {
		defer drains.Done()
		defer func() { _ = errR.Close() }()
		_, copyErr := io.Copy(&p.stderr, errR)
		if copyErr != nil {
			slog.Warn("Pi stderr drain failed", "error", copyErr)
		}
	}()
	go func() { p.waitErr = cmd.Wait(); drains.Wait(); close(p.done) }()
	return p, nil
}

func (p *process) send(data []byte) <-chan error {
	result := make(chan error, 1)
	p.writers.Add(1)
	go func() { defer p.writers.Done(); _, err := p.stdin.Write(data); result <- err }()
	return result
}

type exchange struct {
	process *process
	written <-chan error
}

func (e *exchange) finishWrite(ctx context.Context) error {
	if e.written == nil {
		return ctx.Err()
	}
	select {
	case err := <-e.written:
		e.written = nil
		if err != nil {
			return fmt.Errorf("write Pi command: %w", err)
		}
		return ctx.Err()
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (e *exchange) next(ctx context.Context) (outputLine, error) {
	if err := ctx.Err(); err != nil {
		return outputLine{}, err
	}
	for {
		select {
		case <-ctx.Done():
			return outputLine{}, ctx.Err()
		case err := <-e.written:
			e.written = nil
			if err != nil {
				return outputLine{}, fmt.Errorf("write Pi command: %w", err)
			}
		case line, ok := <-e.process.lines:
			if !ok {
				return outputLine{}, fmt.Errorf("pi stdout closed before operation completed")
			}
			if line.err != nil {
				return outputLine{}, fmt.Errorf("read Pi stdout: %w", line.err)
			}
			return line, nil
		}
	}
}

func (p *process) stop(grace time.Duration) error {
	close(p.discard)
	_ = p.stdin.Close()
	timer := time.NewTimer(grace)
	defer timer.Stop()
	var forced bool
	var killErr error
	select {
	case <-p.done:
	case <-timer.C:
		forced = true
		slog.Warn("Pi shutdown grace expired; killing process group", "pid", p.cmd.Process.Pid)
		killErr = syscall.Kill(-p.cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(killErr, syscall.ESRCH) {
			killErr = nil
		}
		<-p.done
	}
	p.writers.Wait()
	if p.stderr.size > 0 {
		level := slog.LevelDebug
		versionPreflight := len(p.cmd.Args) == 2 && p.cmd.Args[1] == "--version"
		if p.operationFailed || versionPreflight && p.waitErr != nil {
			level = slog.LevelError
		} else if forced || p.waitErr != nil {
			level = slog.LevelWarn
		}
		slog.Log(context.Background(), level, "Pi process stderr diagnostics", "pid", p.cmd.Process.Pid,
			"stderr_tail", string(p.stderr.data[:p.stderr.size]), "stderr_bytes", p.stderr.total,
			"stderr_truncated", p.stderr.total > int64(p.stderr.size), "version_preflight", versionPreflight,
			"operation_failed", p.operationFailed, "forced", forced, "exit_error", p.waitErr)
	}
	slog.Info("Pi process reaped and pipes drained", "pid", p.cmd.Process.Pid, "forced", forced)
	var warnings []error
	if forced {
		warnings = append(warnings, fmt.Errorf("pi required forced termination after shutdown grace"))
	}
	if killErr != nil {
		warnings = append(warnings, fmt.Errorf("kill Pi process group: %w", killErr))
	}
	if p.waitErr != nil {
		warnings = append(warnings, fmt.Errorf("pi process exit: %w", p.waitErr))
	}
	return errors.Join(warnings...)
}
