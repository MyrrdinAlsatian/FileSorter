package utils

import (
	"flag"
	"os"
	"testing"
)

func TestParseFlagsYes(t *testing.T) {
	oldArgs := os.Args
	oldCommandLine := flag.CommandLine
	t.Cleanup(func() {
		os.Args = oldArgs
		flag.CommandLine = oldCommandLine
	})

	// ParseFlags utilise le FlagSet global; le remplacer isole ce test des autres flags du processus.
	flag.CommandLine = flag.NewFlagSet("filesorter-test", flag.ContinueOnError)
	os.Args = []string{"filesorter", "--yes"}

	options := ParseFlags()
	if !options.Yes {
		t.Fatal("--yes was not parsed")
	}
}
