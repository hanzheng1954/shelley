// Command lazycue runs self-healing browser tests described in plain English.
//
// Usage:
//
//	lazycue [options] "test description" ["test description" ...]
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	lazycue "github.com/boldsoftware/shelley/lazycue"
)

func main() {
	runTests()
}

func runTests() {
	baseURL := flag.String("base-url", "", "Base URL of the app under test (required)")
	testsFile := flag.String("tests-file", "", "Read test descriptions from a JSON file (array of strings); combined with any positional args")
	cacheDir := flag.String("cache-dir", "", "Directory for cache JSON files (default: .lazycue next to --tests-file, else .lazycue)")
	model := flag.String("model", "", "LLM model (default: claude-sonnet-4-6)")
	apiURL := flag.String("api-url", "", "Anthropic API base URL (env: ANTHROPIC_BASE_URL)")
	apiKey := flag.String("api-key", "", "Anthropic API key (env: ANTHROPIC_API_KEY)")
	verbose := flag.Bool("verbose", false, "Verbose output")

	flag.Parse()

	if *baseURL == "" {
		fmt.Fprintln(os.Stderr, "error: --base-url is required")
		os.Exit(2)
	}

	descriptions := flag.Args()
	if *testsFile != "" {
		fromFile, err := readTestsFile(*testsFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: reading tests file: %v\n", err)
			os.Exit(2)
		}
		descriptions = append(descriptions, fromFile...)
	}
	if len(descriptions) == 0 {
		fmt.Fprintln(os.Stderr, `usage: lazycue [options] "test description" ["test description" ...]`)
		fmt.Fprintln(os.Stderr, "       lazycue --tests-file tests.json [options]")
		os.Exit(2)
	}

	resolvedCacheDir := *cacheDir
	if resolvedCacheDir == "" {
		if *testsFile != "" {
			resolvedCacheDir = filepath.Join(filepath.Dir(*testsFile), ".lazycue")
		} else {
			resolvedCacheDir = ".lazycue"
		}
	}

	opts := lazycue.Options{
		BaseURL:          *baseURL,
		CacheDir:         resolvedCacheDir,
		Model:            *model,
		AnthropicBaseURL: *apiURL,
		AnthropicAPIKey:  *apiKey,
		Verbose:          *verbose,
	}

	ctx := context.Background()
	var anyFailed bool
	suiteStart := time.Now()

	for i, desc := range descriptions {
		if i > 0 {
			fmt.Println()
		}
		result, err := lazycue.Run(ctx, opts, desc)
		if err != nil {
			printError(i+1, len(descriptions), desc, err)
			anyFailed = true
			continue
		}

		printResult(i+1, len(descriptions), result)

		if !result.Pass {
			anyFailed = true
		}
	}

	// Suite summary.
	if len(descriptions) > 1 {
		fmt.Println()
		elapsed := time.Since(suiteStart).Round(time.Millisecond)
		if anyFailed {
			fmt.Printf("\033[31m✗ some tests failed\033[0m  (%s)\n", elapsed)
		} else {
			fmt.Printf("\033[32m✓ %d tests passed\033[0m  (%s)\n", len(descriptions), elapsed)
		}
	}

	if anyFailed {
		os.Exit(1)
	}
}

// readTestsFile reads a JSON array of test description strings from a file.
func readTestsFile(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var descs []string
	if err := json.Unmarshal(data, &descs); err != nil {
		return nil, fmt.Errorf("expected a JSON array of strings: %w", err)
	}
	return descs, nil
}

func printResult(idx, total int, r *lazycue.TestResult) {
	// Status emoji + colour.
	var status, colour, reset string
	if r.Pass {
		status = "PASS"
		colour = "\033[32m" // green
	} else {
		status = "FAIL"
		colour = "\033[31m" // red
	}
	reset = "\033[0m"

	// Mode badge.
	var badge string
	switch r.Mode {
	case lazycue.RunModeCached:
		badge = fmt.Sprintf("cached v%d", r.CacheVersion)
	case lazycue.RunModeGenerated:
		badge = fmt.Sprintf("generated → v%d", r.CacheVersion)
	case lazycue.RunModeHealed:
		badge = fmt.Sprintf("healed → v%d", r.CacheVersion)
	}

	// Timing.
	totalMs := r.TotalDuration.Round(time.Millisecond)
	var timing string
	if r.AgentDuration > 0 {
		timing = fmt.Sprintf("%s total, %s agent", totalMs, r.AgentDuration.Round(time.Millisecond))
	} else {
		timing = totalMs.String()
	}

	// Header line.
	if total > 1 {
		fmt.Printf("%s%s%s  %d/%d  [%s]  %s\n", colour, status, reset, idx, total, badge, timing)
	} else {
		fmt.Printf("%s%s%s  [%s]  %s\n", colour, status, reset, badge, timing)
	}

	// Description (dimmed).
	fmt.Printf("\033[2m  %s\033[0m\n", truncateDesc(r.Description, 120))

	// Steps.
	if len(r.Steps) > 0 {
		for _, s := range r.Steps {
			mark := "\033[32m✓\033[0m"
			if !s.Pass {
				mark = "\033[31m✗\033[0m"
			}
			line := fmt.Sprintf("  %s %-50s %6s", mark, s.Summary, s.Duration.Round(time.Millisecond))
			if s.Error != "" {
				line += fmt.Sprintf("  \033[31m%s\033[0m", truncateDesc(s.Error, 80))
			}
			fmt.Println(line)
		}
	}

	// Token usage.
	if r.InputTokens > 0 {
		fmt.Printf("\033[2m  ⚡ %s in / %s out tokens  ~$%.3f\033[0m\n", formatTokens(r.InputTokens), formatTokens(r.OutputTokens), r.EstimatedCost)
	}

	// Error detail for failures.
	if !r.Pass && r.Error != "" {
		errLines := strings.Split(r.Error, "\n")
		if len(errLines) <= 3 {
			fmt.Printf("\033[31m  %s\033[0m\n", r.Error)
		} else {
			for _, l := range errLines[:3] {
				fmt.Printf("\033[31m  %s\033[0m\n", l)
			}
			fmt.Printf("\033[2m  ... (%d more lines)\033[0m\n", len(errLines)-3)
		}
	}
}

func printError(idx, total int, desc string, err error) {
	if total > 1 {
		fmt.Printf("\033[31mERROR\033[0m  %d/%d\n", idx, total)
	} else {
		fmt.Printf("\033[31mERROR\033[0m\n")
	}
	fmt.Printf("\033[2m  %s\033[0m\n", truncateDesc(desc, 120))
	fmt.Printf("\033[31m  %s\033[0m\n", err)
}

func truncateDesc(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

// formatTokens formats an integer with comma separators: 14832 → "14,832".
func formatTokens(n int) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	var result []byte
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(c))
	}
	return string(result)
}
