//go:build !unix

package input

import (
	"os"
	"time"
)

// pendingStdin cannot inspect stdin without reading it on this platform, so
// the caller falls back to a one-byte read.
func pendingStdin(*os.File, time.Duration) (pending, ok bool, err error) {
	return false, false, nil
}
