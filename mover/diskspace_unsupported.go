//go:build !windows && !linux && !darwin && !dragonfly && !freebsd && !netbsd && !openbsd

package mover

import "fmt"

func availableDiskSpace(path string) (uint64, error) {
	return 0, fmt.Errorf("disk-space checks are not supported on this operating system")
}
