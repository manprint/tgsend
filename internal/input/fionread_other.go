//go:build unix && !(linux || darwin || dragonfly || freebsd || netbsd || openbsd)

package input

// fionread is unknown here; a readable descriptor is then checked by reading.
const fionread = 0
