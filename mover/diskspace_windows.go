//go:build windows

package mover

import "golang.org/x/sys/windows"

func availableDiskSpace(path string) (uint64, error) {
	windowsPath, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}

	var available uint64
	if err := windows.GetDiskFreeSpaceEx(windowsPath, &available, nil, nil); err != nil {
		return 0, err
	}
	return available, nil
}
