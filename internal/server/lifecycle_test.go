package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// The fixture is the test binary, never a host OpenCode installation or a model.
func TestProcessFixture(t *testing.T) {
	if os.Getenv("GCM_SERVER_FIXTURE") != "1" {
		return
	}
	dir := os.Getenv("GCM_SERVER_FIXTURE_DIR")
	mark := func(name string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("1"), 0o600); err != nil {
			panic(err)
		}
	}
	has := func(name string) bool {
		_, err := os.Stat(filepath.Join(dir, name))
		return err == nil
	}
	mode := os.Getenv("GCM_SERVER_FIXTURE_MODE")
	if strings.HasPrefix(mode, "descendant") {
		binary, err := os.Executable()
		if err != nil {
			panic(err)
		}
		child := exec.Command(binary, "-test.run=^TestProcessDescendantFixture$")
		child.Stdout, child.Stderr = os.Stdout, os.Stderr
		if err := child.Start(); err != nil {
			panic(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "descendant-pid"), []byte(strconv.Itoa(child.Process.Pid)), 0o600); err != nil {
			panic(err)
		}
		for !has("descendant-ready") {
			time.Sleep(time.Millisecond)
		}
	}
	term := make(chan os.Signal, 1)
	signal.Notify(term, syscall.SIGTERM)
	if mode == "bad-health" {
		fmt.Println("opencode server listening on http://127.0.0.1:0")
	} else if !strings.HasPrefix(mode, "no-url") {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			panic(err)
		}
		go func() {
			_ = http.Serve(listener, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))
		}()
		fmt.Printf("opencode server listening on http://%s\n", listener.Addr())
	}
	mark("ready")
	for {
		select {
		case <-term:
			mark("term")
			if strings.HasSuffix(mode, "ignore") {
				continue
			}
			if mode == "signal" {
				signal.Stop(term)
				_ = syscall.Kill(os.Getpid(), syscall.SIGTERM)
				select {}
			}
			if mode == "held" {
				for !has("release") {
					time.Sleep(time.Millisecond)
				}
			}
			if mode == "long-output" {
				fmt.Println(strings.Repeat("x", 128*1024))
			}
			for range 4096 {
				fmt.Println(strings.Repeat("drained output ", 32))
			}
			fmt.Fprintln(os.Stderr, "fixture stderr drained")
			mark("finished")
			if mode == "bad-exit" {
				os.Exit(7)
			}
			os.Exit(0)
		default:
			if has("exit") {
				fmt.Fprintln(os.Stderr, "fixture prior exit")
				mark("finished")
				if mode == "prior-clean" || mode == "descendant-release" {
					os.Exit(0)
				}
				if mode == "prior-signal" {
					signal.Stop(term)
					_ = syscall.Kill(os.Getpid(), syscall.SIGTERM)
					select {}
				}
				os.Exit(9)
			}
			time.Sleep(time.Millisecond)
		}
	}
}

func fixtureServer(t *testing.T, mode string) (*ProcessServer, string) {
	t.Helper()
	dir := t.TempDir()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(dir, "opencode")
	if err := os.WriteFile(executable, []byte("#!/bin/sh\nexec \"$GCM_SERVER_FIXTURE_BINARY\" -test.run=^TestProcessFixture$\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GORACE", "atexit_sleep_ms=0")
	t.Setenv("GCM_SERVER_FIXTURE", "1")
	t.Setenv("GCM_SERVER_FIXTURE_DIR", dir)
	t.Setenv("GCM_SERVER_FIXTURE_MODE", mode)
	t.Setenv("GCM_SERVER_FIXTURE_BINARY", binary)
	t.Setenv("PATH", dir)
	s := New(executable)
	t.Cleanup(func() {
		if s.cmd != nil && s.cmd.Process != nil {
			_ = syscall.Kill(-s.cmd.Process.Pid, syscall.SIGKILL)
		}
		_ = s.Stop()
	})
	return s, dir
}

func awaitMarker(t *testing.T, dir, name string) {
	t.Helper()
	deadline := time.After(10 * time.Second)
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	for {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("fixture did not reach %s", name)
		case <-tick.C:
		}
	}
}

func releaseFixture(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), nil, 0o600); err != nil {
		t.Fatal(err)
	}
}

func assertReaped(t *testing.T, s *ProcessServer) {
	t.Helper()
	select {
	case <-s.done:
	default:
		t.Fatal("Stop returned before wait and output drain completed")
	}
	if s.cmd.ProcessState == nil {
		t.Fatal("Stop returned before process wait completed")
	}
	if err := s.cmd.Process.Signal(syscall.Signal(0)); !errors.Is(err, os.ErrProcessDone) {
		t.Fatalf("process not reaped: %v", err)
	}
}

func TestProcessServerStopGracefulOrderAndDrain(t *testing.T) {
	s, dir := fixtureServer(t, "held")
	if _, err := s.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	stopped := make(chan error, 1)
	go func() { stopped <- s.Stop() }()
	awaitMarker(t, dir, "term")
	select {
	case err := <-stopped:
		t.Fatalf("Stop returned before fixture completed graceful shutdown: %v", err)
	default:
	}
	releaseFixture(t, dir, "release")
	if err := <-stopped; err != nil {
		t.Fatalf("graceful stop: %v", err)
	}
	awaitMarker(t, dir, "finished")
	assertReaped(t, s)
	if err := s.Stop(); err != nil {
		t.Fatalf("repeated stop: %v", err)
	}
}

func TestProcessServerStopRequestedSignal(t *testing.T) {
	s, dir := fixtureServer(t, "signal")
	if _, err := s.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	awaitMarker(t, dir, "ready")
	if err := s.Stop(); err != nil {
		t.Fatalf("requested SIGTERM is not a cleanup failure: %v", err)
	}
	awaitMarker(t, dir, "term")
	assertReaped(t, s)
}

func TestProcessServerStopForcedDiagnostic(t *testing.T) {
	s, dir := fixtureServer(t, "ignore")
	s.stopGrace = 100 * time.Millisecond
	if _, err := s.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	awaitMarker(t, dir, "ready")
	err := s.Stop()
	awaitMarker(t, dir, "term")
	if err == nil || !strings.Contains(err.Error(), "forc") {
		t.Fatalf("forced termination must produce a cleanup diagnostic: %v", err)
	}
	assertReaped(t, s)
}

func TestProcessServerStopWaitFailure(t *testing.T) {
	s, dir := fixtureServer(t, "bad-exit")
	if _, err := s.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	awaitMarker(t, dir, "ready")
	err := s.Stop()
	if err == nil || !strings.Contains(err.Error(), "exit status 7") {
		t.Fatalf("lost abnormal wait result: %v", err)
	}
	assertReaped(t, s)
}

func TestProcessServerStartCancellationCompletesCleanup(t *testing.T) {
	s, dir := fixtureServer(t, "no-url")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan error, 1)
	go func() {
		_, err := s.Start(ctx)
		started <- err
	}()
	awaitMarker(t, dir, "ready")
	cancel()
	err := <-started
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("lost startup cancellation: %v", err)
	}
	awaitMarker(t, dir, "term")
	awaitMarker(t, dir, "finished")
	assertReaped(t, s)
}

func TestProcessServerStartMissingExecutable(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, err := New("opencode").Start(context.Background())
	if !errors.Is(err, ErrOpenCodeNotFound) || !errors.Is(err, exec.ErrNotFound) {
		t.Fatalf("missing executable diagnostic: %v", err)
	}
}

func TestProcessServerStopObservedPriorExit(t *testing.T) {
	for _, mode := range []string{"prior-clean", "prior-signal", "prior-error"} {
		t.Run(mode, func(t *testing.T) {
			s, dir := fixtureServer(t, mode)
			if _, err := s.Start(context.Background()); err != nil {
				t.Fatal(err)
			}
			releaseFixture(t, dir, "exit")
			select {
			case <-s.done:
			case <-time.After(10 * time.Second):
				t.Fatal("fixture exit was not observed")
			}
			err := s.Stop()
			if err == nil || !strings.Contains(err.Error(), "before stop request") {
				t.Fatalf("lost observed prior exit: %v", err)
			}
			if mode == "prior-error" && !strings.Contains(err.Error(), "exit status 9") {
				t.Fatalf("lost prior wait failure: %v", err)
			}
			if mode == "prior-signal" && !strings.Contains(err.Error(), "terminated") {
				t.Fatalf("an unrequested prior SIGTERM must not be suppressed: %v", err)
			}
			assertReaped(t, s)
		})
	}
}

func TestProcessServerStartFailureCompletesCleanup(t *testing.T) {
	for _, mode := range []string{"no-url", "bad-health"} {
		t.Run(mode, func(t *testing.T) {
			s, dir := fixtureServer(t, mode)
			started := make(chan error, 1)
			go func() {
				_, err := s.Start(context.Background())
				started <- err
			}()
			awaitMarker(t, dir, "ready")
			if mode == "no-url" {
				releaseFixture(t, dir, "exit")
			}
			err := <-started
			if err == nil {
				t.Fatal("expected startup failure")
			}
			if mode == "no-url" {
				if !errors.Is(err, ErrServerExited) || !strings.Contains(err.Error(), "fixture prior exit") || !strings.Contains(err.Error(), "exit status 9") {
					t.Fatalf("lost primary startup error, wait failure or drained stderr: %v", err)
				}
			} else {
				if !strings.Contains(err.Error(), "health check") {
					t.Fatalf("lost health failure: %v", err)
				}
				awaitMarker(t, dir, "term")
			}
			awaitMarker(t, dir, "finished")
			assertReaped(t, s)
		})
	}
}

func TestProcessServerStartupCancellationPreservesForcedDiagnostic(t *testing.T) {
	s, dir := fixtureServer(t, "no-url-ignore")
	s.stopGrace = 100 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan error, 1)
	go func() {
		_, err := s.Start(ctx)
		started <- err
	}()
	awaitMarker(t, dir, "ready")
	cancel()
	err := <-started
	if !errors.Is(err, context.Canceled) || !strings.Contains(err.Error(), "forced termination") {
		t.Fatalf("lost primary cancellation or cleanup diagnostic: %v", err)
	}
	awaitMarker(t, dir, "term")
	assertReaped(t, s)
}

func TestProcessServerCancellationAfterStartUsesExplicitStop(t *testing.T) {
	s, dir := fixtureServer(t, "held")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if _, err := s.Start(ctx); err != nil {
		t.Fatal(err)
	}
	cancel()
	releaseFixture(t, dir, "release")
	if err := s.Stop(); err != nil {
		t.Fatalf("cancellation must not race graceful shutdown: %v", err)
	}
	awaitMarker(t, dir, "term")
	awaitMarker(t, dir, "finished")
	assertReaped(t, s)
}

func TestProcessDescendantFixture(t *testing.T) {
	if os.Getenv("GCM_SERVER_FIXTURE") != "1" {
		return
	}
	dir := os.Getenv("GCM_SERVER_FIXTURE_DIR")
	term := make(chan os.Signal, 1)
	signal.Notify(term, syscall.SIGTERM)
	if err := os.WriteFile(filepath.Join(dir, "descendant-ready"), nil, 0o600); err != nil {
		panic(err)
	}
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-term:
			if err := os.WriteFile(filepath.Join(dir, "descendant-term"), nil, 0o600); err != nil {
				panic(err)
			}
		case <-tick.C:
			if _, err := os.Stat(filepath.Join(dir, "descendant-release")); err == nil {
				if err := os.WriteFile(filepath.Join(dir, "descendant-released"), nil, 0o600); err != nil {
					panic(err)
				}
				os.Exit(0)
			}
		}
	}
}

func TestProcessServerStopEscalatesGroupForDescendantHeldPipes(t *testing.T) {
	s, dir := fixtureServer(t, "descendant")
	s.stopGrace = 500 * time.Millisecond
	if _, err := s.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	awaitMarker(t, dir, "descendant-ready")
	stopped := make(chan error, 1)
	go func() { stopped <- s.Stop() }()
	var err error
	select {
	case err = <-stopped:
	case <-time.After(2 * time.Second):
		_ = syscall.Kill(-s.cmd.Process.Pid, syscall.SIGKILL)
		t.Fatalf("Stop blocked on descendant-held pipes after direct server exit (cleanup result: %v)", <-stopped)
	}
	if err == nil || !strings.Contains(err.Error(), "forced termination") {
		t.Fatalf("incomplete descendant-held drain must require escalation: %v", err)
	}
	awaitMarker(t, dir, "term")
	awaitMarker(t, dir, "finished")
	if _, err := os.Stat(filepath.Join(dir, "descendant-term")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("graceful SIGTERM must target only the server, not its descendant: %v", err)
	}
	assertReaped(t, s)
	if !s.cmd.ProcessState.Success() {
		t.Fatalf("direct server did not exit gracefully before descendant escalation: %v", s.cmd.ProcessState)
	}
	pidBytes, err := os.ReadFile(filepath.Join(dir, "descendant-pid"))
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(string(pidBytes))
	if err != nil {
		t.Fatal(err)
	}
	// The orphan is reaped by the OS adopter, not by ProcessServer's direct-child Wait.
	deadline := time.After(10 * time.Second)
	for !errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) {
		select {
		case <-deadline:
			t.Fatal("descendant survived group escalation or was not reaped by its adopter")
		case <-time.After(time.Millisecond):
		}
	}
}

func TestProcessServerUsesSelectedExecutablePath(t *testing.T) {
	s, dir := fixtureServer(t, "signal")
	t.Setenv("PATH", t.TempDir())
	if _, err := s.Start(context.Background()); err != nil {
		t.Fatalf("selected executable must not require another PATH lookup: %v", err)
	}
	awaitMarker(t, dir, "ready")
	if err := s.Stop(); err != nil {
		t.Fatal(err)
	}
	assertReaped(t, s)
}

func TestProcessServerDrainsUnparsedOutputAfterListenURL(t *testing.T) {
	s, dir := fixtureServer(t, "long-output")
	if _, err := s.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	awaitMarker(t, dir, "ready")
	if err := s.Stop(); err != nil {
		t.Fatalf("post-startup output must be drained without a line-size parser limit: %v", err)
	}
	awaitMarker(t, dir, "finished")
	assertReaped(t, s)
}

func TestProcessServerStopReapedChildWhileDescendantDrains(t *testing.T) {
	s, dir := fixtureServer(t, "descendant-release")
	if _, err := s.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	awaitMarker(t, dir, "descendant-ready")
	releaseFixture(t, dir, "exit")
	awaitMarker(t, dir, "finished")

	// Process.Signal synchronizes with Process.Wait without reading Cmd's wait fields.
	deadline := time.After(10 * time.Second)
	for !errors.Is(s.cmd.Process.Signal(syscall.Signal(0)), os.ErrProcessDone) {
		select {
		case <-deadline:
			t.Fatal("direct child was not reaped before Stop")
		case <-time.After(time.Millisecond):
		}
	}
	select {
	case <-s.done:
		t.Fatal("descendant did not retain the output pipes")
	default:
	}

	// Release within grace; the returned sentinel verifies Stop saw the reaped child,
	// rather than taking the already-completed wait/drain path after release.
	released := make(chan error, 1)
	timer := time.AfterFunc(100*time.Millisecond, func() {
		released <- os.WriteFile(filepath.Join(dir, "descendant-release"), nil, 0o600)
	})
	defer timer.Stop()
	err := s.Stop()
	if releaseErr := <-released; releaseErr != nil {
		t.Fatal(releaseErr)
	}
	awaitMarker(t, dir, "descendant-released")
	assertReaped(t, s)
	if !s.cmd.ProcessState.Success() {
		t.Fatalf("direct child did not exit successfully: %v", s.cmd.ProcessState)
	}
	if !errors.Is(err, os.ErrProcessDone) {
		t.Fatalf("lost already-reaped process diagnostic while pipes drained: %v", err)
	}
	if strings.Contains(err.Error(), "forced termination") {
		t.Fatalf("descendant released pipes within grace; escalation was unnecessary: %v", err)
	}
	for _, marker := range []string{"term", "descendant-term"} {
		if _, statErr := os.Stat(filepath.Join(dir, marker)); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("unexpected signal receipt marker %s: %v", marker, statErr)
		}
	}
}
