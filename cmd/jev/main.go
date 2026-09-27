// Command jev asks TypeSafe's Jev model typed questions from the command line.
package main

import (
	"os"

	"github.com/therealbill/typesafe-go/internal/cli"
)

func main() {
	os.Exit(cli.Main(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
