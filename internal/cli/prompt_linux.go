//go:build linux

package cli

import "golang.org/x/sys/unix"

const promptGetTermios = unix.TCGETS
const promptSetTermios = unix.TCSETS
