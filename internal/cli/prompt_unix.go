//go:build darwin || linux

package cli

import (
	"context"
	"errors"
	"io"
	"os"

	"golang.org/x/sys/unix"
)

// readTerminalPrompt has no reader goroutine: cancellation cannot leave a
// blocked read behind after the terminal mode has been restored.
func readTerminalPrompt(ctx context.Context, input *os.File, secret bool, announce func()) (line string, readErr error) {
	fd := int(input.Fd())
	flags, err := unix.FcntlInt(input.Fd(), unix.F_GETFL, 0)
	if err != nil {
		return "", err
	}
	if _, err := unix.FcntlInt(input.Fd(), unix.F_SETFL, flags|unix.O_NONBLOCK); err != nil {
		return "", err
	}
	defer func() {
		if _, err := unix.FcntlInt(input.Fd(), unix.F_SETFL, flags); err != nil {
			readErr = errors.Join(readErr, err)
		}
	}()
	if secret {
		old, err := unix.IoctlGetTermios(fd, promptGetTermios)
		if err != nil {
			return "", err
		}
		mode := *old
		mode.Lflag &^= unix.ECHO
		mode.Lflag |= unix.ICANON | unix.ISIG
		mode.Iflag |= unix.ICRNL
		if err := unix.IoctlSetTermios(fd, promptSetTermios, &mode); err != nil {
			return "", err
		}
		defer func() {
			if err := unix.IoctlSetTermios(fd, promptSetTermios, old); err != nil {
				readErr = errors.Join(readErr, err)
			}
		}()
	}

	announce()
	reader := &pollPromptReader{ctx: ctx, fd: fd}
	return readPromptLine(reader)
}

type pollPromptReader struct {
	ctx context.Context
	fd  int
}

func (r *pollPromptReader) Read(p []byte) (int, error) {
	for {
		if err := r.ctx.Err(); err != nil {
			return 0, err
		}
		fds := []unix.PollFd{{Fd: int32(r.fd), Events: unix.POLLIN}}
		_, err := unix.Poll(fds, 50)
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			return 0, err
		}
		if fds[0].Revents == 0 {
			continue
		}
		if fds[0].Revents&unix.POLLNVAL != 0 {
			return 0, os.ErrInvalid
		}
		n, err := unix.Read(r.fd, p)
		if errors.Is(err, unix.EINTR) || errors.Is(err, unix.EAGAIN) {
			continue
		}
		if n == 0 && err == nil {
			return 0, io.EOF
		}
		return n, err
	}
}
