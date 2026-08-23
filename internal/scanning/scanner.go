// Package scanning detects credentials and other secrets before they land
// on the server. Patterns are deliberately high-signal to keep false
// positives manageable; operators extend them via configuration.
package scanning

import (
	"fmt"
	"regexp"
	"strings"
)

// Finding describes one detected secret.
type Finding struct {
	RuleID  string `json:"rule_id"`
	Description string `json:"description"`
	Preview string `json:"preview"` // redacted context, never the full match
}

// Rule is one named detector.
type Rule struct {
	ID          string
	Pattern     *regexp.Regexp
	Description string
}

// DefaultRules covers the most commonly leaked credential formats.
func DefaultRules() []Rule {
	return []Rule{
		{"aws-access-key", regexp.MustCompile(`AKIA[0-9A-Z]{16}`), "AWS access key id"},
		{"github-token", regexp.MustCompile(`gh[pousr]_[A-Za-z0-9]{36,255}`), "GitHub token"},
		{"slack-token", regexp.MustCompile(`xox[baprs]-[A-Za-z0-9-]{10,}`), "Slack token"},
		{"private-key", regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`), "embedded private key"},
		{"generic-api-key", regexp.MustCompile(`(?i)(api[_-]?key|secret|password|token)\s*[=:]\s*['"][A-Za-z0-9/_+=-]{16,}['"]`), "hardcoded credential assignment"},
		{"google-api-key", regexp.MustCompile(`AIza[0-9A-Za-z\-_]{35}`), "Google API key"},
	}
}

// Scanner applies rules to content.
type Scanner struct {
	rules []Rule
}

// NewScanner returns a scanner with the default rule set plus extras.
func NewScanner(extra ...Rule) *Scanner {
	rules := append(DefaultRules(), extra...)
	return &Scanner{rules: rules}
}

// ScanText returns findings in a single blob of text.
func (s *Scanner) ScanText(content string) []Finding {
	findings := []Finding{}
	for _, rule := range s.rules {
		matches := rule.Pattern.FindAllString(content, -1)
		if len(matches) == 0 {
			continue
		}
		seen := map[string]bool{}
		for _, match := range matches {
			if seen[match] {
				continue
			}
			seen[match] = true
			findings = append(findings, Finding{
				RuleID:      rule.ID,
				Description: rule.Description,
				Preview:     redact(match),
			})
		}
	}
	return findings
}

// ScanPatch scans a unified diff but only examines added lines (+ prefix),
// so historical content already on the server does not re-alarm.
func (s *Scanner) ScanPatch(patch string) []Finding {
	var added strings.Builder
	for _, line := range strings.Split(patch, "\n") {
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			added.WriteString(line[1:])
			added.WriteString("\n")
		}
	}
	if added.Len() == 0 {
		return nil
	}
	return s.ScanText(added.String())
}

// redact keeps a short prefix so humans can find the line without the
// scanner itself leaking the full secret into logs or reports.
func redact(match string) string {
	const maxVisible = 6
	runes := []rune(match)
	if len(runes) <= maxVisible {
		return strings.Repeat("*", len(runes))
	}
	return string(runes[:maxVisible]) + strings.Repeat("*", min(len(runes)-maxVisible, 12)) + "…"
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

var _ = fmt.Sprintf // reserved for future error wrapping
