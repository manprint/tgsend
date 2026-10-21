//go:build darwin || dragonfly || freebsd || netbsd || openbsd

package input

// fionread is FIONREAD, _IOR('f', 127, int), which reports how many bytes are
// ready to read.
const fionread = 0x4004667f
