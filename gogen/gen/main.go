package main

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/tschottdorf/goplay/gogen"
)

// +build generate

func main() {
	str := gogen.MyCode()
	if err := ioutil.WriteFile(os.Args[1], []byte(str), 0644); err != nil {
		fmt.Fprint(os.Stderr, err.Error())
		os.Exit(1)
	}
}
