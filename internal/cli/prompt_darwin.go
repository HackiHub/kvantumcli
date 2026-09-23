//go:build darwin

package cli

import "golang.org/x/sys/unix"

const promptGetTermios = unix.TIOCGETA
const promptSetTermios = unix.TIOCSETA
