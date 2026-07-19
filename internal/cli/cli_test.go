package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunShowsGenerateHelpAfterArgumentError(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run(nil, &stdout, &stderr)

	if exitCode != 2 {
		t.Fatalf("got exit code %d, want 2", exitCode)
	}
	if stdout.Len() != 0 {
		t.Fatalf("unexpected stdout: %q", stdout.String())
	}
	output := stderr.String()
	if !strings.Contains(output, "pc: error: aspect ratio is required") {
		t.Fatalf("missing argument error: %q", output)
	}
	if !strings.Contains(output, "usage: pc [-h]") {
		t.Fatalf("missing generate usage: %q", output)
	}
	if !strings.Contains(output, "--long-edge") {
		t.Fatalf("missing generate options: %q", output)
	}
}

func TestRunShowsEditHelpAfterArgumentError(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run([]string{"edit"}, &stdout, &stderr)

	if exitCode != 2 {
		t.Fatalf("got exit code %d, want 2", exitCode)
	}
	output := stderr.String()
	if !strings.Contains(output, "pc: error: input image is required") {
		t.Fatalf("missing argument error: %q", output)
	}
	if !strings.Contains(output, "usage: pc edit [-h]") {
		t.Fatalf("missing edit usage: %q", output)
	}
	if !strings.Contains(output, "--input-fidelity") {
		t.Fatalf("missing edit options: %q", output)
	}
}
