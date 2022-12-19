package main

import (
	"net/http"

	"github.com/mr-joshcrane/shellspy"
)

func main() {
	http.HandleFunc("/", shellspy.RemoteShell)
	http.ListenAndServe(":8090", nil)
}
