package placeholderdump

import (
	"strings"
	"testing"

	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
)

func TestPlaceholderDumpUnmarshalCaddyfile(t *testing.T) {
	t.Parallel()

	input := `placeholder_dump {
		content "Request: {http.request.uri}"
		file "/tmp/out.log"
		file_permissions 600
		logger_suffix trace
	}`

	var m PlaceholderDump
	if err := m.UnmarshalCaddyfile(caddyfile.NewTestDispenser(input)); err != nil {
		t.Fatalf("UnmarshalCaddyfile() returned error: %v", err)
	}

	if m.Content != "Request: {http.request.uri}" {
		t.Fatalf("Content = %q, want %q", m.Content, "Request: {http.request.uri}")
	}
	if m.File != "/tmp/out.log" {
		t.Fatalf("File = %q, want %q", m.File, "/tmp/out.log")
	}
	if m.FilePermissions != "600" {
		t.Fatalf("FilePermissions = %q, want %q", m.FilePermissions, "600")
	}
	if m.LoggerSuffix != "trace" {
		t.Fatalf("LoggerSuffix = %q, want %q", m.LoggerSuffix, "trace")
	}
}

func TestPlaceholderDumpUnmarshalCaddyfileErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{
			name: "missing file argument",
			input: `placeholder_dump {
				file
			}`,
			wantErr: "wrong argument count or unexpected line ending after 'file'",
		},
		{
			name: "unknown option",
			input: `placeholder_dump {
				unknown value
			}`,
			wantErr: "unknown option: unknown",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var m PlaceholderDump
			err := m.UnmarshalCaddyfile(caddyfile.NewTestDispenser(tt.input))
			if err == nil {
				t.Fatalf("UnmarshalCaddyfile() error = nil, want %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("UnmarshalCaddyfile() error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}
