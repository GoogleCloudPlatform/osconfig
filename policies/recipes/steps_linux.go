// +build linux

package recipes

import (
	"golang.org/x/sys/unix"
)

func mkCharDevice(path string, devMajor, devMinor uint32) error {
	return unix.Mknod(path, unix.S_IFCHR, int(unix.Mkdev(devMajor, devMinor)))
}

func mkBlockDevice(path string, devMajor, devMinor uint32) error {
	return unix.Mknod(path, unix.S_IFBLK, int(unix.Mkdev(devMajor, devMinor)))
}

func mkFifo(path string, mode uint32) error {
	return unix.Mkfifo(path, mode)
}

func createDefaultEnvironment() ([]string, error) {
	return []string{}, nil
}
