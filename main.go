package main

import (
	"fmt"
	"os"

	"github.com/mobile-next/mobilecli/cli"
)

func main() {
	err := cli.Execute()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
