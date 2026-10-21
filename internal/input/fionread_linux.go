//go:build linux

package input

import "golang.org/x/sys/unix"

// fionread is the ioctl that reports how many bytes are ready to read.
const fionread = unix.TIOCINQ
