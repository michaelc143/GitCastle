package scanning

import (
	"regexp"
	"strings"
	"testing"
)

func TestScanTextDetectsKnownSecrets(t *testing.T) {
	scanner := NewScanner()
	content := `
	normal line
	aws_key = AKIAIOSFODNN7EXAMPLE
	github: ghp_abcdefghijklmnopqrstuvwxyz0123456789ABCDEF
	slack xoxb-123456789012-ABCDEF
	-----BEGIN RSA PRIVATE KEY-----
	google AIzaSyA1234567890abcdefghijklmnopqrstuvwx
	password = "supersecretvalue12345"
	`
	findings := scanner.ScanText(content)

	ruleIDs := map[string]bool{}
	for _, finding := range findings {
		ruleIDs[finding.RuleID] = true
	}
	for _, want := range []string{"aws-access-key", "github-token", "slack-token", "private-key", "google-api-key", "generic-api-key"} {
		if !ruleIDs[want] {
			t.Fatalf("missing rule %s in findings %+v", want, findings)
		}
	}
}

func TestScanTextCleanContentHasNoFindings(t *testing.T) {
	scanner := NewScanner()
	content := "# Castle\n\nJust documentation, no secrets here.\n"
	if findings := scanner.ScanText(content); len(findings) != 0 {
		t.Fatalf("false positives: %+v", findings)
	}
}

func TestScanPatchOnlyFlagsAddedLines(t *testing.T) {
	scanner := NewScanner()
	patch := `diff --git a/config.txt b/config.txt
index abc..def 100644
--- a/config.txt
+++ b/config.txt
-old_password = "removedoldsecret12345"
+new_password = "brandnewsecret12345"
 unchanged context AKIAIOSFODNN7EXAMPLE stays quiet`
	findings := scanner.ScanPatch(patch)
	if len(findings) != 1 {
		t.Fatalf("findings = %d, want 1 (only the added line): %+v", len(findings), findings)
	}
	if !strings.Contains(findings[0].Preview, "*") {
		t.Fatal("preview must be redacted")
	}
}

func TestPreviewNeverContainsFullSecret(t *testing.T) {
	scanner := NewScanner()
	secret := "AKIAIOSFODNN7EXAMPLE"
	findings := scanner.ScanText("key=" + secret)
	if len(findings) == 0 {
		t.Fatal("expected a finding")
	}
	if strings.Contains(findings[0].Preview, secret[len(secret)-4:]) {
		t.Fatalf("preview leaks secret tail: %q", findings[0].Preview)
	}
}

func TestDuplicateMatchesReportedOnce(t *testing.T) {
	scanner := NewScanner()
	content := "AKIAIOSFODNN7EXAMPLE\nAKIAIOSFODNN7EXAMPLE\nAKIAIOSFODNN7EXAMPLE\n"
	if findings := scanner.ScanText(content); len(findings) != 1 {
		t.Fatalf("deduplication failed: %+v", findings)
	}
}

func TestCustomRulesAppend(t *testing.T) {
	extra := Rule{
		ID:          "corp-prefix",
		Pattern:     regexp.MustCompile(`CORP-[A-Z0-9]{20,}`),
		Description: "internal corp token",
	}
	scanner := NewScanner(extra)
	findings := scanner.ScanText("token CORP-ABCDEFGHIJ1234567890")
	found := false
	for _, finding := range findings {
		if finding.RuleID == "corp-prefix" {
			found = true
		}
	}
	if !found {
		t.Fatal("custom rule not applied")
	}
}
