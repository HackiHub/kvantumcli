//go:build darwin || linux

package cli

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestSecretPromptPTYRestoresEcho(t *testing.T) {
	master, slave := openPromptPTY(t)
	before, err := unix.IoctlGetTermios(int(slave.Fd()), promptGetTermios)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	type result struct {
		value string
		err   error
	}
	done := make(chan result, 1)
	go func() { value, err := readTerminalPrompt(ctx, slave, true, func() {}); done <- result{value, err} }()
	waitHidden := func() {
		t.Helper()
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			state, err := unix.IoctlGetTermios(int(slave.Fd()), promptGetTermios)
			if err == nil && state.Lflag&unix.ECHO == 0 {
				return
			}
			time.Sleep(time.Millisecond)
		}
		t.Fatal("secret mode did not activate")
	}
	waitHidden()
	if _, err := master.Write([]byte("synthetic-token\n")); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-done:
		if got.err != nil || got.value != "synthetic-token" {
			t.Fatalf("prompt=%+v", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("secret read did not finish")
	}
	state, err := unix.IoctlGetTermios(int(slave.Fd()), promptGetTermios)
	if err != nil || *state != *before {
		t.Fatalf("terminal not restored: %v", err)
	}
	var output [256]byte
	n := 0
	fds := []unix.PollFd{{Fd: int32(master.Fd()), Events: unix.POLLIN}}
	if ready, _ := unix.Poll(fds, 20); ready > 0 {
		n, _ = unix.Read(int(master.Fd()), output[:])
	}
	if strings.Contains(string(output[:n]), "synthetic-token") {
		t.Fatalf("secret echoed: %q", output[:n])
	}

	done = make(chan result, 1)
	go func() { value, err := readTerminalPrompt(ctx, slave, true, func() {}); done <- result{value, err} }()
	waitHidden()
	cancel()
	select {
	case got := <-done:
		if !errors.Is(got.err, context.Canceled) {
			t.Fatalf("cancellation: %v", got.err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("cancellation left reader blocked")
	}
	state, err = unix.IoctlGetTermios(int(slave.Fd()), promptGetTermios)
	if err != nil || *state != *before {
		t.Fatalf("terminal not restored after cancel: %v", err)
	}
}

func TestOrdinaryPromptPTYCancel(t *testing.T) {
	_, slave := openPromptPTY(t)
	before, err := unix.IoctlGetTermios(int(slave.Fd()), promptGetTermios)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := readTerminalPrompt(ctx, slave, false, func() {}); done <- err }()
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancellation: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("ordinary read did not cancel")
	}
	state, err := unix.IoctlGetTermios(int(slave.Fd()), promptGetTermios)
	if err != nil || *state != *before {
		t.Fatalf("terminal state changed: %v", err)
	}
}

func TestConfigurePTYChild(t *testing.T) {
	if os.Getenv("KVANTUMCI_TEST_PTY_CHILD") != "1" {
		return
	}
	os.Args = []string{"kvantumci", "configure", "--api-url", "https://api.example", "--no-verify"}
	Execute()
	os.Exit(0)
}

func startConfigurePTY(t *testing.T, configPath string) (*os.File, *os.File, *exec.Cmd) {
	t.Helper()
	master, slave := openPromptPTY(t)
	cmd := exec.Command(os.Args[0], "-test.run=^TestConfigurePTYChild$")
	for _, entry := range os.Environ() {
		if strings.HasPrefix(entry, "KVANTUMCI_API_URL=") || strings.HasPrefix(entry, "KVANTUMCI_TOKEN=") || strings.HasPrefix(entry, "KVANTUMCI_TENANT_ID=") || strings.HasPrefix(entry, "KVANTUMCI_CONFIG=") {
			continue
		}
		cmd.Env = append(cmd.Env, entry)
	}
	cmd.Env = append(cmd.Env, "KVANTUMCI_TEST_PTY_CHILD=1", "KVANTUMCI_CONFIG="+configPath)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = slave, slave, slave
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill() })
	return master, slave, cmd
}

func waitForPTYText(t *testing.T, master *os.File, wanted string) string {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	var output strings.Builder
	for time.Now().Before(deadline) {
		fds := []unix.PollFd{{Fd: int32(master.Fd()), Events: unix.POLLIN}}
		ready, err := unix.Poll(fds, 50)
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if ready == 0 {
			continue
		}
		var b [256]byte
		n, err := unix.Read(int(master.Fd()), b[:])
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if n > 0 {
			output.Write(b[:n])
		}
		if strings.Contains(output.String(), wanted) {
			return output.String()
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	t.Fatalf("timed out waiting for %q; output=%q", wanted, output.String())
	return ""
}

func waitForChild(t *testing.T, cmd *exec.Cmd, master *os.File) error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	deadline := time.Now().Add(3 * time.Second)
	var output strings.Builder
	for time.Now().Before(deadline) {
		select {
		case err := <-done:
			return err
		default:
		}
		fds := []unix.PollFd{{Fd: int32(master.Fd()), Events: unix.POLLIN}}
		if ready, _ := unix.Poll(fds, 20); ready > 0 && fds[0].Revents&unix.POLLIN != 0 {
			var b [512]byte
			n, _ := unix.Read(int(master.Fd()), b[:])
			output.Write(b[:n])
		}
	}
	cmd.Process.Kill()
	t.Fatalf("prompt child did not exit; output=%q", output.String())
	return nil
}

func TestConfigureProcessPTYSecretAndInterrupt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	master, _, child := startConfigurePTY(t, path)
	out := waitForPTYText(t, master, "PAT/GAT token:")
	if _, err := master.Write([]byte("synthetic-token\n")); err != nil {
		t.Fatal(err)
	}
	out += waitForPTYText(t, master, "Tenant ID:")
	if strings.Contains(out, "synthetic-token") {
		t.Fatalf("secret echoed: %q", out)
	}
	if _, err := master.Write([]byte("tenant-1\n")); err != nil {
		t.Fatal(err)
	}
	out += waitForPTYText(t, master, "configPath")
	if err := waitForChild(t, child, master); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(data), "synthetic-token") {
		t.Fatalf("config not saved: %v %q", err, data)
	}

	// A partial API flag must still prompt for the missing token.
	if err := os.WriteFile(path, []byte(`{"apiUrl":"https://api.example","tenantId":"tenant-1"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	beforeBytes, _ := os.ReadFile(path)
	before := string(beforeBytes)
	master, _, child = startConfigurePTY(t, path)
	waitForPTYText(t, master, "PAT/GAT token:")
	if _, err := master.Write([]byte{3}); err != nil {
		t.Fatal(err)
	}
	if err := waitForChild(t, child, master); err == nil {
		t.Fatal("interrupt unexpectedly succeeded")
	}
	state, err := unix.IoctlGetTermios(int(master.Fd()), promptGetTermios)
	if err != nil || state.Lflag&unix.ECHO == 0 {
		t.Fatalf("echo not restored after interrupt: %v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != before {
		t.Fatalf("config changed on interrupt: %v", err)
	}
}
