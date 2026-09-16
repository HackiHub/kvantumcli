//go:build windows

package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"
	"unicode/utf16"
	"unicode/utf8"
	"unsafe"

	"golang.org/x/sys/windows"
)

const consoleKeyEvent = 0x0001
const consoleReadNoWait = 0x0002

var readConsoleInputEx = windows.NewLazySystemDLL("kernel32.dll").NewProc("ReadConsoleInputExW")

// A Win32 INPUT_RECORD containing a KEY_EVENT_RECORD (20 bytes total).
type consoleInputRecord struct {
	EventType uint16
	_         uint16
	KeyDown   int32
	Repeat    uint16
	Virtual   uint16
	Scan      uint16
	Char      uint16
	Control   uint32
}

func readTerminalPrompt(ctx context.Context, input *os.File, secret bool, announce func()) (line string, readErr error) {
	h := windows.Handle(input.Fd())
	var old uint32
	if err := windows.GetConsoleMode(h, &old); err != nil {
		return "", err
	}
	mode := old &^ (windows.ENABLE_ECHO_INPUT | windows.ENABLE_LINE_INPUT)
	mode |= windows.ENABLE_PROCESSED_INPUT
	if err := windows.SetConsoleMode(h, mode); err != nil {
		return "", err
	}
	defer func() {
		if err := windows.SetConsoleMode(h, old); err != nil {
			readErr = errors.Join(readErr, err)
		}
	}()

	if err := readConsoleInputEx.Find(); err != nil {
		return "", err
	}
	announce()
	var text []rune
	var pendingHigh uint16
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		var record consoleInputRecord
		var count uint32
		success, _, callErr := readConsoleInputEx.Call(uintptr(h), uintptr(unsafe.Pointer(&record)), 1, uintptr(unsafe.Pointer(&count)), consoleReadNoWait)
		if success == 0 {
			return "", callErr
		}
		if count == 0 {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-ticker.C:
			}
			continue
		}
		if record.EventType != consoleKeyEvent || record.KeyDown == 0 {
			continue
		}
		done, err := appendConsoleKey(&text, &pendingHigh, record, secret)
		if err != nil {
			return "", err
		}
		if done {
			return string(text), nil
		}
	}
}

func appendConsoleKey(text *[]rune, pendingHigh *uint16, record consoleInputRecord, secret bool) (bool, error) {
	for i := uint16(0); i < record.Repeat; i++ {
		c := record.Char
		switch c {
		case 0:
			continue
		case '\r', '\n':
			if !secret {
				fmt.Fprintln(os.Stderr)
			}
			return true, nil
		case '\b':
			if len(*text) > 0 {
				*text = (*text)[:len(*text)-1]
				if !secret {
					fmt.Fprint(os.Stderr, "\b \b")
				}
			}
			continue
		case 3:
			return false, context.Canceled
		}
		var r rune
		if c >= 0xD800 && c <= 0xDBFF {
			*pendingHigh = c
			continue
		} else if c >= 0xDC00 && c <= 0xDFFF {
			if *pendingHigh == 0 {
				continue
			}
			r = utf16.DecodeRune(rune(*pendingHigh), rune(c))
			*pendingHigh = 0
		} else {
			*pendingHigh = 0
			r = rune(c)
		}
		if r == utf8.RuneError || r < ' ' {
			continue
		}
		if len([]byte(string(*text)))+utf8.RuneLen(r) > 4096 {
			return false, fmt.Errorf("input too long")
		}
		*text = append(*text, r)
		if !secret {
			fmt.Fprint(os.Stderr, string(r))
		}
	}
	return false, nil
}
