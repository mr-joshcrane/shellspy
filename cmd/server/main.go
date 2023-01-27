package main

import (
	"fmt"
	"os"

	"github.com/mr-joshcrane/shellspy"
)

func main() {
	PORT := os.Getenv("PORT")
	PASSWORD := os.Getenv("PASSWORD")
	fmt.Println("Starting shellspy on port", PORT)
	err := shellspy.ListenAndServe(fmt.Sprintf("0.0.0.0:%s", PORT), shellspy.NewPassword(PASSWORD), os.Stdout)
	panic(err)
}

// logging connections
// server startup diags
// transcript -> to some logs
