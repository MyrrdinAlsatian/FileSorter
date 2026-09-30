//go:build linux || darwin || dragonfly || freebsd || netbsd || openbsd

package mover

import (
	"fmt"

	"golang.org/x/sys/unix"
)

func availableDiskSpace(path string) (uint64, error) {
	var stats unix.Statfs_t
	if err := unix.Statfs(path, &stats); err != nil {
		return 0, err
	}
	if stats.Bsize < 0 || stats.Bavail < 0 {
		return 0, fmt.Errorf("filesystem returned negative free-space values")
	}

	blockSize := uint64(stats.Bsize)
	availableBlocks := uint64(stats.Bavail)
	if blockSize != 0 && availableBlocks > ^uint64(0)/blockSize {
		return ^uint64(0), nil
	}
	return availableBlocks * blockSize, nil
}
