package shell

import (
	"regexp"
	"strings"

	"github.com/Dxrk777/Dxrk/internal/strconst"
)

type ValidationResult struct {
	IsValid   bool
	Reason    string
	RiskLevel string
	Warnings  []string
}

var dangerousPatterns = []struct {
	Pattern   *regexp.Regexp
	RiskLevel string
	Reason    string
}{
	{regexp.MustCompile(`rm\s+(-[a-zA-Z]*r[a-zA-Z]*f|--recursive\s+--force|-rf|-fr|-R[a-zA-Z]*f|-f[a-zA-Z]*r)\s+/(\s|$)`), strconst.StrCritical, "recursive force delete on root"},
	{regexp.MustCompile(`rm\s+(-[a-zA-Z]*r[a-zA-Z]*f|--recursive\s+--force|-rf|-fr)\s+~`), "high", "recursive force delete on home"},
	{regexp.MustCompile(`mkfs\.`), strconst.StrCritical, "filesystem format"},
	{regexp.MustCompile(`dd\s+.*of=/dev/`), strconst.StrCritical, "raw disk write"},
	{regexp.MustCompile(`>\s*/dev/sd[a-z]`), strconst.StrCritical, "raw disk write via redirect"},
	{regexp.MustCompile(`:()\s*\{\s*:\|:\s*&\s*\};\s*:`), strconst.StrCritical, "fork bomb"},
	{regexp.MustCompile(`chmod\s+(-R\s+)?777\s+/(\s|$)`), "high", "world-writable root permissions"},
	{regexp.MustCompile(`curl\s+.*\|\s*(ba)?sh`), "high", "remote code execution via pipe"},
	{regexp.MustCompile(`wget\s+.*\|\s*(ba)?sh`), "high", "remote code execution via pipe"},
	{regexp.MustCompile(`eval\s+`), strconst.StrMedium2, "dynamic code evaluation"},
	{regexp.MustCompile(`\$\(`), "low", "command substitution"},
	{regexp.MustCompile(`sudo\s+`), "low", "elevated privileges"},
	{regexp.MustCompile(`chmod\s+`), "low", "permission change"},
	{regexp.MustCompile(`chown\s+`), "low", "ownership change"},
}

var (
	singleDangerous = []string{
		"rm -rf /",
		"rm -rf /*",
		"rm -rf ~",
		":(){:|:&};:",
		"mkfs.ext4",
		"mkfs.xfs",
		"mkfs.btrfs",
		"dd if=/dev/zero of=/dev/sda",
	}
)

func ValidateCommand(input string) *ValidationResult {
	result := &ValidationResult{
		IsValid:   true,
		RiskLevel: "none",
		Warnings:  []string{},
	}

	input = strings.TrimSpace(input)
	if input == "" {
		result.IsValid = false
		result.Reason = "empty command"
		return result
	}

	lower := strings.ToLower(input)

	for _, dangerous := range singleDangerous {
		if strings.Contains(lower, dangerous) {
			result.IsValid = false
			result.Reason = "dangerous command detected: " + dangerous
			result.RiskLevel = strconst.StrCritical
			return result
		}
	}

	for _, dp := range dangerousPatterns {
		if dp.Pattern.MatchString(input) {
			switch dp.RiskLevel {
			case strconst.StrCritical:
				result.IsValid = false
				result.Reason = dp.Reason
				result.RiskLevel = strconst.StrCritical
				return result
			case "high":
				result.Warnings = append(result.Warnings, dp.Reason)
				if result.RiskLevel != strconst.StrCritical {
					result.RiskLevel = "high"
				}
			case strconst.StrMedium2:
				result.Warnings = append(result.Warnings, dp.Reason)
				if result.RiskLevel == "none" || result.RiskLevel == "low" {
					result.RiskLevel = strconst.StrMedium2
				}
			case "low":
				result.Warnings = append(result.Warnings, dp.Reason)
				if result.RiskLevel == "none" {
					result.RiskLevel = "low"
				}
			}
		}
	}

	return result
}
