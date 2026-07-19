package cli

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	minPixels       = 655_360
	maxPixels       = 8_294_400
	maxEdge         = 3_840
	maxRatio        = 3
	defaultLongEdge = 1_536
)

type commandKind int

const (
	generateCommand commandKind = iota
	editCommand
)

type options struct {
	kind          commandKind
	aspectRatio   [2]float64
	imagePath     string
	prompt        string
	promptFile    string
	outputDir     string
	name          string
	format        string
	model         string
	quality       string
	longEdge      int
	size          string
	inputFidelity string
	compression   *int
	help          bool
}

func defaultOptions(kind commandKind) options {
	return options{
		kind:      kind,
		outputDir: ".",
		name:      "image",
		format:    "png",
		model:     "gpt-image-2",
		quality:   "auto",
		longEdge:  defaultLongEdge,
		size:      "auto",
	}
}

func parseArgs(arguments []string) (options, error) {
	kind := generateCommand
	if len(arguments) > 0 && arguments[0] == "edit" {
		kind = editCommand
		arguments = arguments[1:]
	}
	parsed := defaultOptions(kind)
	positionals := make([]string, 0, 2)
	optionsEnded := false
	for index := 0; index < len(arguments); index++ {
		argument := arguments[index]
		if optionsEnded {
			positionals = append(positionals, argument)
			continue
		}
		if argument == "--" {
			optionsEnded = true
			continue
		}
		if argument == "-h" || argument == "--help" {
			parsed.help = true
			continue
		}
		if !strings.HasPrefix(argument, "-") || argument == "-" {
			positionals = append(positionals, argument)
			continue
		}

		name, inlineValue := splitOption(argument)
		value, nextIndex, err := optionValue(name, inlineValue, arguments, index)
		if err != nil {
			return options{}, err
		}
		index = nextIndex
		if err := applyOption(&parsed, name, value); err != nil {
			return options{}, err
		}
	}
	if parsed.help {
		return parsed, nil
	}
	if err := applyPositionals(&parsed, positionals); err != nil {
		return options{}, err
	}
	return parsed, validateOptions(parsed)
}

func splitOption(argument string) (string, *string) {
	name, value, found := strings.Cut(argument, "=")
	if !found {
		return argument, nil
	}
	return name, &value
}

func optionValue(name string, inline *string, arguments []string, index int) (string, int, error) {
	if inline != nil {
		return *inline, index, nil
	}
	if index+1 >= len(arguments) {
		return "", index, fmt.Errorf("%s requires a value", name)
	}
	return arguments[index+1], index + 1, nil
}

func applyOption(parsed *options, name, value string) error {
	switch name {
	case "-p", "--prompt-file":
		parsed.promptFile = value
	case "-o", "--output-dir":
		parsed.outputDir = value
	case "-n", "--name":
		parsed.name = value
	case "-f", "--format":
		parsed.format = value
	case "--model":
		parsed.model = value
	case "--quality":
		parsed.quality = value
	case "--long-edge":
		if parsed.kind == editCommand {
			return fmt.Errorf("unknown option: %s", name)
		}
		longEdge, err := strconv.Atoi(value)
		if err != nil {
			return errors.New("--long-edge must be an integer")
		}
		parsed.longEdge = longEdge
	case "--size":
		if parsed.kind == generateCommand {
			return fmt.Errorf("unknown option: %s", name)
		}
		parsed.size = value
	case "--input-fidelity":
		if parsed.kind == generateCommand {
			return fmt.Errorf("unknown option: %s", name)
		}
		parsed.inputFidelity = value
	case "--compression":
		compression, err := strconv.Atoi(value)
		if err != nil {
			return errors.New("--compression must be an integer")
		}
		parsed.compression = &compression
	default:
		return fmt.Errorf("unknown option: %s", name)
	}
	return nil
}

func applyPositionals(parsed *options, values []string) error {
	if len(values) > 2 {
		return errors.New("too many positional arguments")
	}
	if parsed.kind == editCommand {
		if len(values) == 0 {
			return errors.New("input image is required")
		}
		parsed.imagePath = values[0]
		if len(values) == 2 {
			parsed.prompt = values[1]
		}
		return nil
	}
	if len(values) == 0 {
		return errors.New("aspect ratio is required")
	}
	ratio, err := parseAspectRatio(values[0])
	if err != nil {
		return err
	}
	parsed.aspectRatio = ratio
	if len(values) == 2 {
		parsed.prompt = values[1]
	}
	return nil
}

func validateOptions(parsed options) error {
	if parsed.prompt == "" && parsed.promptFile == "" {
		return errors.New("must provide prompt text or --prompt-file")
	}
	if parsed.format != "png" && parsed.format != "webp" {
		return errors.New("--format must be png or webp")
	}
	if !isOneOf(parsed.quality, "low", "medium", "high", "auto") {
		return errors.New("--quality must be low, medium, high, or auto")
	}
	if parsed.inputFidelity != "" && !isOneOf(parsed.inputFidelity, "low", "high") {
		return errors.New("--input-fidelity must be low or high")
	}
	if parsed.compression != nil && (*parsed.compression < 0 || *parsed.compression > 100) {
		return errors.New("--compression must be between 0 and 100")
	}
	if parsed.kind == editCommand {
		info, err := os.Stat(expandHome(parsed.imagePath))
		if err != nil || !info.Mode().IsRegular() {
			return fmt.Errorf("image file does not exist: %s", parsed.imagePath)
		}
	}
	return nil
}

func isOneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func parseAspectRatio(value string) ([2]float64, error) {
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return [2]float64{}, errors.New("aspect ratio must be W:H, e.g. 16:9 or 2.35:1")
	}
	width, widthErr := strconv.ParseFloat(parts[0], 64)
	height, heightErr := strconv.ParseFloat(parts[1], 64)
	if widthErr != nil || heightErr != nil {
		return [2]float64{}, errors.New("aspect ratio must contain only numbers and a colon")
	}
	if width <= 0 || height <= 0 {
		return [2]float64{}, errors.New("both width and height must be greater than 0")
	}
	if math.Max(width, height)/math.Min(width, height) > maxRatio {
		return [2]float64{}, errors.New("OpenAI Image API requires aspect ratio no greater than 3:1")
	}
	return [2]float64{width, height}, nil
}

func sizeFromRatio(ratio [2]float64, longEdge int) (string, error) {
	if longEdge <= 0 || longEdge%16 != 0 {
		return "", errors.New("--long-edge must be a positive multiple of 16")
	}
	if longEdge > maxEdge {
		return "", errors.New("--long-edge cannot exceed 3840")
	}
	width, height := longEdge, longEdge
	if ratio[0] >= ratio[1] {
		height = roundToMultipleOf16(float64(longEdge) * ratio[1] / ratio[0])
	} else {
		width = roundToMultipleOf16(float64(longEdge) * ratio[0] / ratio[1])
	}
	pixels := width * height
	if pixels < minPixels {
		return "", fmt.Errorf("computed size %dx%d is below the Image API minimum pixel count", width, height)
	}
	if pixels > maxPixels {
		return "", fmt.Errorf("computed size %dx%d exceeds the Image API maximum pixel count", width, height)
	}
	return fmt.Sprintf("%dx%d", width, height), nil
}

func roundToMultipleOf16(value float64) int {
	return max(16, int(math.RoundToEven(value/16))*16)
}

func readPrompt(prompt, promptFile string) (string, error) {
	path := promptFile
	if path == "" {
		if info, err := os.Stat(prompt); err == nil && info.Mode().IsRegular() {
			path = prompt
		}
	}
	text := prompt
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read prompt file: %w", err)
		}
		text = string(data)
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return "", errors.New("prompt must not be empty")
	}
	return text, nil
}

func outputPath(directory, filename, format string) string {
	directory = expandHome(directory)
	if filepath.Ext(filename) == "" {
		filename += "." + format
	}
	return filepath.Join(directory, filename)
}

func expandHome(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}
	return path
}
