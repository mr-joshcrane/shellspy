[![Go Reference](https://pkg.go.dev/badge/github.com/mr-joshcrane/shellspy.svg)](https://pkg.go.dev/github.com/mr-joshcrane/shellspy)[![License: GPL-2.0](https://img.shields.io/badge/Licence-GPL-2)](https://opensource.org/licenses/GPL-2.0)[![Go Report Card](https://goreportcard.com/badge/github.com/mr-joshcrane/shellspy)](https://goreportcard.com/report/github.com/mr-joshcrane/shellspy)

# ShellSpy: A simple program that is designed to keep a log of a users shell session.

ShellSpy is available in two flavours.

LocalSpy Mode: Useful when you want to capture a transcript of local shell commands and output.
LocalSpy Quick Install
```bash
$ go install github.com/mr-joshcrane/cmd/shellspy/@latest
$ shellspy
Welcome to the remote shell!
$
```

**ServerSpy Quick Install**
```bash
$ go install github.com/mr-joshcrane/cmd/shellspysrv@latest
$ export PORT=8000
$ export PASSPASSWORD=mySecurePassword
$ shellspysrv
Starting shellspy on port 8000
Starting listener on 0.0.0.0:8000
Listener created.

## Server now listening for connections
```

**Server Quick Connect**
```bash
$ nc localhost 8000
Enter Password:
mySecurePassword
Welcome to the remote shell!
$
```
Transcript of sessions are stored on disk to `transcript.txt`

