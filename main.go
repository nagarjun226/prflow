package main

import (
	"fmt"
	"os"

	"github.com/cheenu1092-oss/prflow/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
