package main

import (
	"github.com/sapcc/argora/internal/cmd"
	"os"
)

func main() {
	err := cmd.RootCmd.Execute()
	if err != nil {
		println(err.Error())
		os.Exit(1)
	}
}
