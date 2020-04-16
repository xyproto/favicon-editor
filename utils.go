package main

import (
	"fmt"
	"os"

	"github.com/xyproto/vt100"
)

// exists checks if the given path exists
func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func quitError(tty *vt100.TTY, err error) {
	if tty != nil {
		tty.Close()
	}
	vt100.Reset()
	vt100.Clear()
	vt100.Close()
	fmt.Fprintln(os.Stderr, "error: "+err.Error())
	vt100.SetXY(uint(0), uint(1))
	os.Exit(1)
}
