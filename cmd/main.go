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
	session := shellspy.NewSpySession()
	session.Transcript = newFile
	session.Start()
}
