//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package maintenance

import (
	"fmt"
	"os"
	"syscall"
)

func Acquire(path string, exclusive bool, nonblocking bool) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	op := syscall.LOCK_SH
	if exclusive {
		op = syscall.LOCK_EX
	}
	if nonblocking {
		op |= syscall.LOCK_NB
	}
	if err = syscall.Flock(int(f.Fd()), op); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("database is in use; stop GeoEntity before this operation")
	}
	return f, nil
}

func Release(f *os.File) {
	if f != nil {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
	}
}
