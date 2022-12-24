package main

import (
	"github.com/mr-joshcrane/shellspy"
)

func main() {
	err := shellspy.ListenAndServe(":8090")
	panic(err)
}
