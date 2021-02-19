package main

import (
	"os"

	"github.com/release-engineering/exodus-rsync/internal/cmd"
)

func main() {
	os.Exit(cmd.Main(os.Args))
}
