package security

import (
	"testing"
)

func TestValidateCommand_PyrightLangserver(t *testing.T) {
	tests := []struct {
		name    string
		command string
		args    []string
		wantErr bool
	}{
		{
			name:    "pyright-langserver allowed",
			command: "pyright-langserver",
			args:    []string{"--stdio"},
			wantErr: false,
		},
		{
			name:    "pyright-langserver with path allowed",
			command: "/usr/local/bin/pyright-langserver",
			args:    []string{"--stdio"},
			wantErr: false,
		},
		{
			name:    "pyright-langserver with home path allowed",
			command: "~/.npm/bin/pyright-langserver",
			args:    []string{"--stdio"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCommand(tt.command, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateCommand() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}