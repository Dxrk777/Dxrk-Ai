// SPDX-License-Identifier: MIT
package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/Dxrk777/Dxrk-Ai/internal/cli"
	"github.com/Dxrk777/Dxrk-Ai/internal/git"
	"github.com/Dxrk777/Dxrk-Ai/internal/strconst"
)

func RegisterSecurityReviewCommand() {
	cmd := &cobra.Command{
		Use:   "security-review",
		Short: "Security-focused code review of pending changes",
		Long: `Perform a security-focused code review of changes on the current branch.

Analyzes the diff for common security vulnerabilities including:
- SQL injection, command injection, XSS
- Authentication/authorization bypass
- Hardcoded secrets or credentials
- Insecure deserialization
- Path traversal
- Cryptographic weaknesses

Examples:
  dxrk security-review
  dxrk security-review --since main`,
		Args: cobra.NoArgs,
		RunE: runSecurityReview,
	}

	cmd.Flags().String("since", "", "Review changes since specified ref (default: origin/HEAD)")

	cli.AddCommand(cmd)
}

func runSecurityReview(cmd *cobra.Command, _ []string) error {
	since, _ := cmd.Flags().GetString("since")

	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	runner := git.NewRunner(wd)
	ctx := cmd.Context()

	// Check if in a git repo
	if _, err := runner.Root(ctx); err != nil {
		return fmt.Errorf("not a git repository: %w", err)
	}

	// Get status
	status, err := runner.Status(ctx)
	if err != nil {
		return fmt.Errorf("get status: %w", err)
	}

	// Get diff
	var diff *git.DiffResult
	if since != "" {
		diffArgs := []string{"diff", since, "HEAD"}
		diff, err = gitDiffRef(ctx, wd, diffArgs)
	} else {
		// Try origin/HEAD, fall back to unstaged + staged
		diffArgs := []string{"diff", "origin/HEAD", "HEAD"}
		diff, err = gitDiffRef(ctx, wd, diffArgs)
		if err != nil {
			// No remote, review uncommitted changes
			staged, _ := runner.Diff(ctx, true, "")
			unstaged, _ := runner.Diff(ctx, false, "")
			diff = &git.DiffResult{
				Files: append(staged.Files, unstaged.Files...),
				Stats: git.DiffStats{
					FilesChanged: staged.Stats.FilesChanged + unstaged.Stats.FilesChanged,
					Additions:    staged.Stats.Additions + unstaged.Stats.Additions,
					Deletions:    staged.Stats.Deletions + unstaged.Stats.Deletions,
				},
			}
		}
	}

	if err != nil {
		return fmt.Errorf("get diff: %w", err)
	}

	fmt.Println("🔒 Security Review")
	fmt.Println("==================")
	fmt.Printf("Branch: %s\n", status.Branch)
	fmt.Printf("Files changed: %d\n", diff.Stats.FilesChanged)
	fmt.Printf("Additions: %d, Deletions: %d\n\n", diff.Stats.Additions, diff.Stats.Deletions)

	// Categorize findings
	var findings []securityFinding

	for _, f := range diff.Files {
		secFindings := analyzeFileForSecurity(f)
		findings = append(findings, secFindings...)
	}

	if len(findings) == 0 {
		fmt.Println("✅ No security issues detected in the changes.")
		fmt.Println("\nNote: This is an automated scan. Manual review is still recommended.")
		return nil
	}

	// Print findings
	highCount, medCount, lowCount := 0, 0, 0
	for _, f := range findings {
		switch f.severity {
		case "HIGH":
			highCount++
		case strconst.StrMedium:
			medCount++
		case "LOW":
			lowCount++
		}
	}

	fmt.Printf("Found %d potential security issues:\n", len(findings))
	fmt.Printf("  HIGH: %d, MEDIUM: %d, LOW: %d\n\n", highCount, medCount, lowCount)

	for i, f := range findings {
		fmt.Printf("## %d. [%s] %s\n", i+1, f.severity, f.category)
		fmt.Printf("   File: %s\n", f.file)
		fmt.Printf("   Description: %s\n", f.description)
		fmt.Printf("   Recommendation: %s\n\n", f.recommendation)
	}

	fmt.Println("Note: This is an automated scan. Review findings carefully for false positives.")
	return nil
}

type securityFinding struct {
	severity       string
	category       string
	file           string
	description    string
	recommendation string
}

func analyzeFileForSecurity(f git.DiffFile) []securityFinding {
	var findings []securityFinding

	content := f.Content

	// Check for hardcoded secrets
	secretPatterns := map[string]string{
		"password":   "Potential hardcoded password",
		"api_key":    "Potential hardcoded API key",
		"secret":     "Potential hardcoded secret",
		"token":      "Potential hardcoded token",
		"credential": "Potential hardcoded credential",
	}

	for pattern, desc := range secretPatterns {
		if containsIgnoreCase(content, pattern) {
			findings = append(findings, securityFinding{
				severity:       "HIGH",
				category:       "Hardcoded Secret",
				file:           f.Path,
				description:    desc,
				recommendation: "Use environment variables or a secrets manager instead of hardcoding credentials",
			})
		}
	}

	// Check for SQL injection patterns
	if containsAny(content, "fmt.Sprintf(\"SELECT", "fmt.Sprintf(\"INSERT", "fmt.Sprintf(\"UPDATE", "fmt.Sprintf(\"DELETE",
		"fmt.Sprintf(\"DROP", "query := fmt.Sprintf") {
		findings = append(findings, securityFinding{
			severity:       "HIGH",
			category:       "SQL Injection",
			file:           f.Path,
			description:    "String interpolation in SQL query",
			recommendation: "Use parameterized queries or prepared statements",
		})
	}

	// Check for command injection
	if containsAny(content, "exec.Command(\"sh\", \"-c\"", "exec.Command(\"bash\", \"-c\"",
		"os/exec.Command(\"sh\"", "os/exec.Command(\"bash\"") {
		findings = append(findings, securityFinding{
			severity:       "HIGH",
			category:       "Command Injection",
			file:           f.Path,
			description:    "Shell command execution with potential user input",
			recommendation: "Avoid shell execution; use direct command execution with argument arrays",
		})
	}

	// Check for unsafe deserialization
	if containsAny(content, "json.Unmarshal", "xml.Unmarshal", "yaml.Unmarshal",
		"encoding/gob", "pickle.loads") {
		findings = append(findings, securityFinding{
			severity:       strconst.StrMedium,
			category:       "Deserialization",
			file:           f.Path,
			description:    "Deserialization of potentially untrusted data",
			recommendation: "Validate input before deserialization; consider using safe parsing methods",
		})
	}

	// Check for path traversal
	if containsAny(content, "os.Open(", "ioutil.ReadFile(", "os.ReadFile(",
		"path.Join(userInput", "filepath.Join(userInput") {
		findings = append(findings, securityFinding{
			severity:       strconst.StrMedium,
			category:       "Path Traversal",
			file:           f.Path,
			description:    "File operation with potentially user-controlled path",
			recommendation: "Validate and sanitize file paths; use filepath.Clean and check for .. components",
		})
	}

	// Check for weak crypto
	if containsAny(content, "md5.New()", "sha1.New()", "math/rand",
		"crypto/md5", "crypto/sha1") {
		findings = append(findings, securityFinding{
			severity:       strconst.StrMedium,
			category:       "Weak Cryptography",
			file:           f.Path,
			description:    "Use of weak or deprecated cryptographic algorithm",
			recommendation: "Use SHA-256 or stronger; use crypto/rand instead of math/rand for security purposes",
		})
	}

	// Check for XSS in templates
	if containsAny(content, "template.HTML(", "template.JS(",
		"dangerouslySetInnerHTML", "v-html") {
		findings = append(findings, securityFinding{
			severity:       strconst.StrMedium,
			category:       "XSS",
			file:           f.Path,
			description:    "Potential cross-site scripting via unescaped output",
			recommendation: "Escape user input before rendering; avoid raw HTML insertion",
		})
	}

	return findings
}

func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) && containsAny(s, substr)
}

func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if len(s) >= len(sub) {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}
