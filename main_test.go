package main

import (
	"bytes"
	"strings"
	"testing"

	"FileRecoveryOrganizer/mover"
)

func TestConfirmMoverExecution(t *testing.T) {
	tests := []struct {
		name           string
		mode           mover.Mode
		overwriteMode  mover.OverwriteMode
		yes            bool
		input          string
		operationCount int
		wantConfirmed  bool
		wantPrompt     bool
	}{
		{name: "copy without overwrite needs no prompt", mode: mover.ModeCopy, overwriteMode: mover.ConflictSkip, operationCount: 3, wantConfirmed: true},
		{name: "move can be declined", mode: mover.ModeMove, overwriteMode: mover.ConflictSkip, input: "n\n", operationCount: 3, wantPrompt: true},
		{name: "French confirmation accepts oui", mode: mover.ModeMove, overwriteMode: mover.ConflictSkip, input: "oui\n", operationCount: 3, wantConfirmed: true, wantPrompt: true},
		{name: "overwrite asks even in copy mode", mode: mover.ModeCopy, overwriteMode: mover.ConflictOverwrite, input: "y\n", operationCount: 3, wantConfirmed: true, wantPrompt: true},
		{name: "yes bypasses the prompt", mode: mover.ModeMove, overwriteMode: mover.ConflictOverwrite, yes: true, operationCount: 3, wantConfirmed: true},
		{name: "no remaining operations needs no prompt", mode: mover.ModeMove, overwriteMode: mover.ConflictOverwrite, operationCount: 0, wantConfirmed: true},
		{name: "end of input is a refusal", mode: mover.ModeMove, overwriteMode: mover.ConflictSkip, operationCount: 3, wantPrompt: true},
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
				test.operationCount,
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
