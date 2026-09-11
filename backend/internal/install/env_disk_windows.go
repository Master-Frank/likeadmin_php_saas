//go:build windows

package install

import "golang.org/x/sys/windows"

func diskFreeBytes(dir string) (float64, error) {
	var free, total, totalFree uint64
	path, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return 0, err
	}
	if err := windows.GetDiskFreeSpaceEx(path, &free, &total, &totalFree); err != nil {
		return 0, err
	}
	return float64(free), nil
}
