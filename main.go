package main

import (
	"os"

	"github.com/loupax/sauce/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
