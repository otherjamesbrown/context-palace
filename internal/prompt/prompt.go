package prompt

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// Reader wraps stdin for interactive prompts.
type Reader struct {
	scanner *bufio.Scanner
	writer  io.Writer
}

// NewReader creates a prompt reader using stdin/stdout.
func NewReader() *Reader {
	return &Reader{
		scanner: bufio.NewScanner(os.Stdin),
		writer:  os.Stdout,
	}
}

// NewReaderWithIO creates a reader with custom input/output (for testing).
func NewReaderWithIO(r io.Reader, w io.Writer) *Reader {
	return &Reader{
		scanner: bufio.NewScanner(r),
		writer:  w,
	}
}

// ReadLine prompts user and returns trimmed input.
func (r *Reader) ReadLine(prompt string) (string, error) {
	fmt.Fprint(r.writer, prompt)
	if !r.scanner.Scan() {
		if err := r.scanner.Err(); err != nil {
			return "", err
		}
		return "", io.EOF
	}
	return strings.TrimSpace(r.scanner.Text()), nil
}

// Confirm prompts for y/n confirmation.
// Returns true for y/Y, false for n/N.
func (r *Reader) Confirm(prompt string) (bool, error) {
	for {
		input, err := r.ReadLine(prompt)
		if err != nil {
			return false, err
		}
		input = strings.ToLower(input)
		if input == "y" || input == "yes" {
			return true, nil
		}
		if input == "n" || input == "no" {
			return false, nil
		}
		fmt.Fprintln(r.writer, "Please enter 'y' or 'n'")
	}
}

// PrintHeader prints the ingest flow header.
func PrintHeader(w io.Writer) {
	fmt.Fprintln(w, "=== ContextPalace: Create Memo from Experience ===")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Answer these questions to create a new memo:")
	fmt.Fprintln(w)
}

// PrintPrompt prints a numbered prompt with guidance.
func PrintPrompt(w io.Writer, num int, label, guidance string) {
	fmt.Fprintf(w, "%d. %s:\n", num, label)
	if guidance != "" {
		fmt.Fprintf(w, "   %s\n", guidance)
	}
	fmt.Fprint(w, "   > ")
}

// IngestInput holds the gathered input from interactive prompts.
type IngestInput struct {
	Category string
	What     string
	Cause    string
	Correct  string
	Trigger  string
}

// RunIngestPrompts runs the interactive ingest flow.
func RunIngestPrompts(existingCategories []string) (*IngestInput, error) {
	r := NewReader()
	w := os.Stdout

	PrintHeader(w)

	// 1. Category
	if len(existingCategories) > 0 {
		fmt.Fprintf(w, "1. CATEGORY: What type of task was this?\n")
		fmt.Fprintf(w, "   (Existing: %s | Or enter new category)\n", strings.Join(existingCategories, ", "))
	} else {
		fmt.Fprintf(w, "1. CATEGORY: What type of task was this?\n")
		fmt.Fprintf(w, "   (Enter a category name, e.g., build, deploy, ci-cd)\n")
	}
	fmt.Fprint(w, "   > ")
	category, err := r.ReadLine("")
	if err != nil {
		return nil, err
	}
	if category == "" {
		return nil, fmt.Errorf("category is required")
	}
	fmt.Fprintln(w)

	// 2. What happened
	fmt.Fprintln(w, "2. WHAT HAPPENED: Describe the CLASS of mistake, not just this instance.")
	fmt.Fprintln(w, "   Think: what general pattern would catch this AND similar issues?")
	fmt.Fprintln(w, "   (Bad: \"go build ./cmd/cli failed\" - too specific)")
	fmt.Fprintln(w, "   (Good: \"Assuming standard Go project layout without checking\" - catches similar issues)")
	fmt.Fprint(w, "   > ")
	what, err := r.ReadLine("")
	if err != nil {
		return nil, err
	}
	if what == "" {
		return nil, fmt.Errorf("description of what happened is required")
	}
	fmt.Fprintln(w)

	// 3. Root cause
	fmt.Fprintln(w, "3. ROOT CAUSE: Why did this type of mistake happen?")
	fmt.Fprint(w, "   > ")
	cause, err := r.ReadLine("")
	if err != nil {
		return nil, err
	}
	if cause == "" {
		return nil, fmt.Errorf("root cause is required")
	}
	fmt.Fprintln(w)

	// 4. Correct approach
	fmt.Fprintln(w, "4. CORRECT APPROACH: What's the general rule to follow?")
	fmt.Fprint(w, "   > ")
	correct, err := r.ReadLine("")
	if err != nil {
		return nil, err
	}
	if correct == "" {
		return nil, fmt.Errorf("correct approach is required")
	}
	fmt.Fprintln(w)

	// 5. Trigger (optional)
	fmt.Fprintln(w, "5. TRIGGER: When should this memo be consulted? (optional)")
	fmt.Fprintln(w, "   (This becomes the CLAUDE.md entry. Press Enter to skip.)")
	fmt.Fprint(w, "   > ")
	trigger, err := r.ReadLine("")
	if err != nil {
		return nil, err
	}
	fmt.Fprintln(w)

	return &IngestInput{
		Category: category,
		What:     what,
		Cause:    cause,
		Correct:  correct,
		Trigger:  trigger,
	}, nil
}
