package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/xingkaixin/pic-create/internal/images"
	"github.com/xingkaixin/pic-create/internal/oauth"
)

func Run(arguments []string, stdout, stderr io.Writer) int {
	parsed, err := parseArgs(arguments)
	if err != nil {
		fmt.Fprintf(stderr, "pc: error: %v\n\n%s", err, helpText(arguments))
		return 2
	}
	if parsed.help {
		fmt.Fprint(stdout, helpText(arguments))
		return 0
	}

	target, err := execute(context.Background(), parsed)
	if err != nil {
		fmt.Fprintf(stderr, "pc: error: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, target)
	return 0
}

func helpText(arguments []string) string {
	if len(arguments) > 0 && arguments[0] == "edit" {
		return editHelp
	}
	return generateHelp
}

func execute(ctx context.Context, parsed options) (string, error) {
	prompt, err := readPrompt(parsed.prompt, parsed.promptFile)
	if err != nil {
		return "", err
	}

	store, err := oauth.CodexStore()
	if err != nil {
		return "", err
	}
	client := images.NewClient(oauth.NewClient(store))
	request := images.Options{
		Model:         parsed.model,
		Prompt:        prompt,
		Quality:       parsed.quality,
		Format:        parsed.format,
		InputFidelity: parsed.inputFidelity,
	}

	var image []byte
	if parsed.kind == editCommand {
		request.Size = parsed.size
		image, err = client.Edit(ctx, expandHome(parsed.imagePath), request)
	} else {
		request.Size, err = sizeFromRatio(parsed.aspectRatio, parsed.longEdge)
		if err == nil {
			image, err = client.Generate(ctx, request)
		}
	}
	if err != nil {
		return "", err
	}

	target := outputPath(parsed.outputDir, parsed.name, parsed.format)
	if err := images.WriteFile(target, image); err != nil {
		return "", err
	}
	return target, nil
}
