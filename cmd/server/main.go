package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/mr-joshcrane/shellspy"
)

type Body struct {
	Body string
}

func remoteShell(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	body, _ := io.ReadAll(r.Body)

	var result Body
	err := json.Unmarshal(body, &result)
	if err != nil {
		fmt.Println("Can not unmarshal JSON")
	}
	command := strings.NewReader(result.Body)
	session := shellspy.SpySession(command, w)
	session.Transcript = os.Stdout
	session.Start()
}

func main() {
	http.HandleFunc("/", remoteShell)
	http.ListenAndServe(":8090", nil)
}
