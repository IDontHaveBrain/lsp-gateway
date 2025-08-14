package security

import (
    "testing"
)

func TestValidateCommand_AllowsWhitelisted(t *testing.T) {
    if err := ValidateCommand("gopls", []string{"serve"}); err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
}

func TestValidateCommand_RejectsUnknown(t *testing.T) {
    if err := ValidateCommand("rm", []string{"-rf", "/"}); err == nil {
        t.Fatalf("expected error for unknown command")
    }
}

func TestValidateCommand_BlocksBasicInjection(t *testing.T) {
    cases := [][]string{{"$(whoami)"}, {";", "rm", "-rf", "/"}}
    for _, args := range cases {
        if err := ValidateCommand("gopls", args); err == nil {
            t.Fatalf("expected rejection for %v", args)
        }
    }
}

func TestValidateCommand_FuzzingStylePatterns(t *testing.T) {
	dangerousPatterns := []string{"|", "&", ";", "`", "$", "$("}
	traversalPatterns := []string{".."}

	tests := []struct {
		name     string
		patterns []string
		prefix   string
		suffix   string
	}{
		{"prefixed dangerous", dangerousPatterns, "prefix", ""},
		{"suffixed dangerous", dangerousPatterns, "", "suffix"},
		{"wrapped dangerous", dangerousPatterns, "pre", "suf"},
		{"prefixed traversal", traversalPatterns, "prefix", ""},
		{"suffixed traversal", traversalPatterns, "", "suffix"},
		{"wrapped traversal", traversalPatterns, "pre", "suf"},
	}

	for _, tt := range tests {
		for _, pattern := range tt.patterns {
			testArg := tt.prefix + pattern + tt.suffix
			t.Run(fmt.Sprintf("%s_%s", tt.name, pattern), func(t *testing.T) {
				if err := ValidateCommand("gopls", []string{testArg}); err == nil {
					t.Errorf("expected pattern %s to be blocked in: %s", pattern, testArg)
				}
			})
		}
	}
}

func TestValidateCommand_BlocksTraversal(t *testing.T) {
	if err := ValidateCommand("gopls", []string{"../../etc/passwd"}); err == nil {
		t.Fatalf("expected traversal to be blocked")
	}
}

func TestValidateCommand_BlocksShellInjection(t *testing.T) {
	cases := [][]string{{"--flag", "a;b"}, {"--opt", "a|b"}, {"--x", "a&b"}, {"--x", "`echo hi`"}, {"--x", "$(whoami)"}, {"--x", "$HOME"}}
	for _, args := range cases {
		if err := ValidateCommand("gopls", args); err == nil {
			t.Fatalf("expected shell injection blocked for args: %v", args)
		}
	}
}

func TestValidateCommand_AdvancedEvasionTechniques(t *testing.T) {
	tests := []struct {
		name string
		args []string
		desc string
	}{
		{"concatenated injection", []string{"--flag=value;whoami"}, "semicolon after equals"},
		{"spaced injection", []string{"arg1", "; whoami"}, "space before semicolon"},
		{"multiple injections", []string{"arg; whoami & cat /etc/passwd"}, "multiple injection patterns"},
		{"injection in middle", []string{"start; whoami; end"}, "injection in middle of argument"},
		{"encoded space", []string{"arg%20;%20whoami"}, "URL encoded spaces with injection"},
		{"tab separated", []string{"arg\t;\twhoami"}, "tab separated injection"},
		{"mixed operators", []string{"cmd | grep pattern && echo done"}, "mixed shell operators"},
		{"chained commands", []string{"ls; cat file; rm temp"}, "chained command execution"},
		{"variable with command", []string{"$USER; whoami"}, "variable followed by command"},
		{"backtick in quotes", []string{"'`whoami`'"}, "backtick within quotes"},
		{"dollar in quotes", []string{"'$(whoami)'"}, "dollar substitution in quotes"},
		{"escaped characters", []string{"\\; whoami"}, "escaped semicolon"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateCommand("gopls", tt.args); err == nil {
				t.Errorf("expected %s to be blocked: %v", tt.desc, tt.args)
			}
		})
	}
}
