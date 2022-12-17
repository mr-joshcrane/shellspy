package main

import (
	"os"

	"github.com/mr-joshcrane/shellspy"
)

func main() {
	newFile, err := os.Create("transcript.txt")
	if err != nil {
		panic(err)
	}
	session := shellspy.SpySession(os.Stdin, os.Stdout)
	session.Transcript = newFile
	session.Start()
}
