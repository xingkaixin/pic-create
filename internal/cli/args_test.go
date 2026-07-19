package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseGenerateArguments(t *testing.T) {
	parsed, err := parseArgs([]string{
		"16:9",
		"a skyline",
		"-o", "out",
		"-n", "skyline",
		"-f", "png",
		"--model", "gpt-image-2",
		"--quality", "high",
		"--long-edge", "1536",
		"--compression", "80",
	})
	if err != nil {
		t.Fatalf("parse generate arguments: %v", err)
	}
	if parsed.kind != generateCommand || parsed.prompt != "a skyline" {
		t.Fatalf("unexpected command: %#v", parsed)
	}
	if parsed.outputDir != "out" || parsed.name != "skyline" {
		t.Fatalf("unexpected output: %#v", parsed)
	}
	if parsed.quality != "high" || parsed.longEdge != 1536 {
		t.Fatalf("unexpected image options: %#v", parsed)
	}
	if parsed.compression == nil || *parsed.compression != 80 {
		t.Fatalf("unexpected compression: %#v", parsed.compression)
	}
}

func TestParseEditArguments(t *testing.T) {
	directory := t.TempDir()
	imagePath := filepath.Join(directory, "input.png")
	if err := os.WriteFile(imagePath, []byte("image"), 0o644); err != nil {
		t.Fatal(err)
	}

	parsed, err := parseArgs([]string{
		"edit",
		imagePath,
		"replace background",
		"--size", "1024x1024",
		"--input-fidelity", "high",
	})
	if err != nil {
		t.Fatalf("parse edit arguments: %v", err)
	}
	if parsed.kind != editCommand || parsed.imagePath != imagePath {
		t.Fatalf("unexpected command: %#v", parsed)
	}
	if parsed.size != "1024x1024" || parsed.inputFidelity != "high" {
		t.Fatalf("unexpected edit options: %#v", parsed)
	}
}

func TestParseArgumentsAfterDoubleDash(t *testing.T) {
	parsed, err := parseArgs([]string{"1:1", "--", "-literal prompt"})
	if err != nil {
		t.Fatalf("parse arguments: %v", err)
	}
	if parsed.prompt != "-literal prompt" {
		t.Fatalf("unexpected prompt: %q", parsed.prompt)
	}
}

func TestSizeFromRatio(t *testing.T) {
	tests := []struct {
		name     string
		ratio    [2]float64
		longEdge int
		want     string
	}{
		{name: "landscape", ratio: [2]float64{16, 9}, longEdge: 1536, want: "1536x864"},
		{name: "portrait", ratio: [2]float64{9, 16}, longEdge: 1536, want: "864x1536"},
		{name: "square", ratio: [2]float64{1, 1}, longEdge: 1536, want: "1536x1536"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := sizeFromRatio(test.ratio, test.longEdge)
			if err != nil {
				t.Fatalf("calculate size: %v", err)
			}
			if got != test.want {
				t.Fatalf("got %s, want %s", got, test.want)
			}
		})
	}
}

func TestReadPromptFromPositionalFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prompt.txt")
	if err := os.WriteFile(path, []byte("  detailed prompt\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	prompt, err := readPrompt(path, "")
	if err != nil {
		t.Fatalf("read prompt: %v", err)
	}
	if prompt != "detailed prompt" {
		t.Fatalf("unexpected prompt: %q", prompt)
	}
}
