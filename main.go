package main

import (
	"fmt"
	"log"
	"meraca/v4l2"
	"os"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "run error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	dev, err := os.OpenFile("/dev/video0", os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer dev.Close()

	fd := dev.Fd()
	log.Printf("opened /dev/video0 successfully, fd=%d", fd)

	if err := v4l2.Start(fd); err != nil {
		return err
	}

	return nil
}
