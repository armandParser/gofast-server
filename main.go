package main

import (
	"runtime"
)

func main() {
	// Set up runtime
	runtime.GOMAXPROCS(runtime.NumCPU())

	// Execute CLI
	Execute()
}
