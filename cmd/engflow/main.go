package main

import (
	"os"

	"engflow/internal/engflow"
)

func main() {
	os.Exit(engflow.Run(os.Args[1:], os.Stdout, os.Stderr))
}
