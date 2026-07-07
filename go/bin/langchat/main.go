package main

import (
	"fmt"
	"os"

	"github.com/JackKCWong/langchat/go/cmd"
)

func main() {
	root := cmd.NewRoot()
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}