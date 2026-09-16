//go:build !darwin && !linux && !windows

package cli

import (
	"context"
	"os"

	"golang.org/x/term"
)

func readTerminalPrompt(ctx context.Context, input *os.File, secret bool, announce func()) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if secret {
		announce()
		b, err := term.ReadPassword(int(input.Fd()))
		return string(b), err
	}
	announce()
	return readPromptLine(input)
}
