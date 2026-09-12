//go:build !windows

package install

import "syscall"

func diskFreeBytes(dir string) (float64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(dir, &st); err != nil {
		return 0, err
	}
	return float64(st.Bavail) * float64(st.Bsize), nil
}
