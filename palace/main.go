package main

import (
	"os"

	"github.com/otherjamesbrown/context-palace/palace/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
