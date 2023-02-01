package main

import (
	"fmt"
	"os"

	"github.com/mr-joshcrane/shellspy"
)

func main() {
	PORT := os.Getenv("PORT")
	if PORT == "" {
		fmt.Println("PORT environment variable must be set")
		os.Exit(1)
	}
	PASSWORD := os.Getenv("PASSWORD")
	if PASSWORD == "" {
		fmt.Println("PASSWORD environment variable must be set")
		os.Exit(1)
	}

	fmt.Println("Starting shellspy on port", PORT)
	err := shellspy.ListenAndServe(fmt.Sprintf("0.0.0.0:%s", PORT), PASSWORD)
	panic(err)
}
