package main

import (
	"bytes"
	"strings"
	"testing"

	"FileRecoveryOrganizer/mover"
)

func TestConfirmMoverExecution(t *testing.T) {
	tests := []struct {
		name          string
		mode          mover.Mode
		overwriteMode string
		yes           bool
		input         string
		wantConfirmed bool
		wantPrompt    bool
	}{
		{name: "copy without overwrite needs no prompt", mode: mover.ModeCopy, overwriteMode: "skip", wantConfirmed: true},
		{name: "move can be declined", mode: mover.ModeMove, overwriteMode: "skip", input: "n\n", wantPrompt: true},
		{name: "French confirmation accepts oui", mode: mover.ModeMove, overwriteMode: "skip", input: "oui\n", wantConfirmed: true, wantPrompt: true},
		{name: "overwrite asks even in copy mode", mode: mover.ModeCopy, overwriteMode: "overwrite", input: "y\n", wantConfirmed: true, wantPrompt: true},
		{name: "yes bypasses the prompt", mode: mover.ModeMove, overwriteMode: "overwrite", yes: true, wantConfirmed: true},
		{name: "end of input is a refusal", mode: mover.ModeMove, overwriteMode: "skip", wantPrompt: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			confirmed, err := confirmMoverExecution(
				strings.NewReader(test.input),
				&output,
				test.mode,
				test.overwriteMode,
				test.yes,
				3,
				"organized",
			)
			if err != nil {
				t.Fatal(err)
			}
			if confirmed != test.wantConfirmed {
				t.Fatalf("confirmed = %t, want %t", confirmed, test.wantConfirmed)
			}
			if prompted := output.Len() > 0; prompted != test.wantPrompt {
				t.Fatalf("prompted = %t, want %t", prompted, test.wantPrompt)
			}
		})
	}
}
