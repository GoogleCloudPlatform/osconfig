// +build windows

package recipes

import "fmt"

func mkCharDevice(path string, devMajor, devMinor uint32) error {
	return fmt.Errorf("Creating a char device is not supported on windows")
}

func mkBlockDevice(path string, devMajor, devMinor uint32) error {
	return fmt.Errorf("Creating a block device is not supported on windows")
}

func mkFifo(path string, mode uint32) error {
	return fmt.Errorf("Creating a fifo is not supported on windows")
}
