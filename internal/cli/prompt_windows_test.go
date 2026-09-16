//go:build windows

package cli

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestConsoleInputRecordLayout(t *testing.T) {
	if got := unsafe.Sizeof(consoleInputRecord{}); got != 20 {
		t.Fatalf("INPUT_RECORD size = %d, want 20", got)
	}
	if err := readConsoleInputEx.Find(); err != nil {
		t.Fatalf("ReadConsoleInputExW unavailable: %v", err)
	}
}

func TestConsoleKeyHandling(t *testing.T) {
	var text []rune
	var high uint16
	press := func(c uint16, repeat uint16) (bool, error) {
		return appendConsoleKey(&text, &high, consoleInputRecord{EventType: consoleKeyEvent, KeyDown: 1, Char: c, Repeat: repeat}, true)
	}
	if done, err := press('a', 2); done || err != nil {
		t.Fatalf("repeat: %v %v", done, err)
	}
	if done, err := press('\b', 1); done || err != nil {
		t.Fatalf("backspace: %v %v", done, err)
	}
	_, _ = press(0xD83D, 1)
	_, _ = press(0xDE00, 1)
	if done, err := press('\r', 1); !done || err != nil || string(text) != "a😀" {
		t.Fatalf("line=%q done=%v err=%v", string(text), done, err)
	}
	if _, err := press(3, 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("Ctrl-C: %v", err)
	}
	text = make([]rune, 4096)
	if _, err := press('x', 1); err == nil {
		t.Fatal("long input accepted")
	}
}

var writeConsoleInput = windows.NewLazySystemDLL("kernel32.dll").NewProc("WriteConsoleInputW")

func openTestConsole(t *testing.T) *os.File {
	t.Helper()
	input, err := os.OpenFile("CONIN$", os.O_RDWR, 0)
	if err != nil {
		t.Fatalf("open isolated console input: %v", err)
	}
	t.Cleanup(func() { input.Close() })
	return input
}

func queueConsoleKeys(t *testing.T, h windows.Handle, chars string) {
	t.Helper()
	if err := writeConsoleInput.Find(); err != nil {
		t.Fatal(err)
	}
	for _, c := range chars {
		record := consoleInputRecord{EventType: consoleKeyEvent, KeyDown: 1, Repeat: 1, Char: uint16(c)}
		var count uint32
		ok, _, callErr := writeConsoleInput.Call(uintptr(h), uintptr(unsafe.Pointer(&record)), 1, uintptr(unsafe.Pointer(&count)))
		if ok == 0 || count != 1 {
			t.Fatalf("WriteConsoleInputW: count=%d err=%v", count, callErr)
		}
	}
}

func TestConsolePromptNativeChild(t *testing.T) {
	if os.Getenv("KVANTUMCI_TEST_CONSOLE_CHILD") != "1" {
		return
	}
	input := openTestConsole(t)
	h := windows.Handle(input.Fd())
	var old uint32
	if err := windows.GetConsoleMode(h, &old); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	announced := false
	line, err := readTerminalPrompt(ctx, input, true, func() {
		announced = true
		var during uint32
		if modeErr := windows.GetConsoleMode(h, &during); modeErr != nil || during&windows.ENABLE_ECHO_INPUT != 0 {
			t.Errorf("echo was enabled when secret prompt became visible: %v", modeErr)
		}
		queueConsoleKeys(t, h, "secret\r")
	})
	if !announced || err != nil || line != "secret" {
		t.Fatalf("announcement=%v line=%q error=%v", announced, line, err)
	}
	var after uint32
	if err := windows.GetConsoleMode(h, &after); err != nil || after != old {
		t.Fatalf("console mode not restored: before=%x after=%x err=%v", old, after, err)
	}
	started := time.Now()
	_, err = readTerminalPrompt(ctx, input, true, func() {
		time.AfterFunc(70*time.Millisecond, cancel)
	})
	if !errors.Is(err, context.Canceled) || time.Since(started) > time.Second {
		t.Fatalf("polling cancellation: %v after %s", err, time.Since(started))
	}
	if err := windows.GetConsoleMode(h, &after); err != nil || after != old {
		t.Fatalf("console mode not restored after cancellation: before=%x after=%x err=%v", old, after, err)
	}
}

func TestConsolePromptNative(t *testing.T) {
	if os.Getenv("KVANTUMCI_TEST_CONSOLE_CHILD") == "1" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestConsolePromptNativeChild$")
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_CONSOLE}
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "KVANTUMCI_TEST_CONSOLE_CHILD=") {
			cmd.Env = append(cmd.Env, entry)
		}
	}
	cmd.Env = append(cmd.Env, "KVANTUMCI_TEST_CONSOLE_CHILD=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("native console child: %v; output=%s", err, out)
	}
}
