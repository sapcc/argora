// Copyright 2024 SAP SE
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"

	"github.com/sapcc/argora/internal/cmd"
)

func main() {
	err := cmd.RootCmd.Execute()
	if err != nil {
		println(err.Error())
		os.Exit(1)
	}
}
