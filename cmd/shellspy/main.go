package main

import (
	"os"

	"github.com/mr-joshcrane/shellspy"
)

func main() {
	os.Exit(shellspy.LocalInstance())
}
