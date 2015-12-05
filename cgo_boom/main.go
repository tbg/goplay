package main

import (
	"os/user"

	"C"
)
import "net"

func main() {
	for i := 0; i < 1000; i++ {
		_, _ = net.Dial("tcp", "localhost:1337")
		_, _ = user.Current()
	}
}
