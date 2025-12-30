package main

import (
	"fmt"
	"log"
	"meraca/v4l2"
	"os"

	"golang.org/x/sys/unix"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "run error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	fd, err := unix.Open("/dev/video0", unix.O_RDWR, 0)
	// dev, err := os.OpenFile("/dev/video0", os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer unix.Close(fd)

	log.Printf("opened /dev/video0 successfully, fd=%d", fd)

	frame, err := v4l2.CaptureOneFrame(uintptr(fd), 4)
	if err != nil {
		return err
	}
	fmt.Printf("frame: %v", frame)

	if err := os.WriteFile("./frame.raw", frame, 0644); err != nil {
		return err
	}

	return nil
}
