package placeholderdump

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

type testNextHandler struct {
	calls int
	err   error
}

func (h *testNextHandler) ServeHTTP(http.ResponseWriter, *http.Request) error {
	h.calls++
	return h.err
}

func TestPlaceholderDumpValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		module  PlaceholderDump
		wantErr string
	}{
		{
			name:    "missing file and logger",
			module:  PlaceholderDump{Content: "value"},
			wantErr: "either file or logger_suffix must be set",
		},
		{
			name:    "missing content",
			module:  PlaceholderDump{File: "out.log"},
			wantErr: "content must be set",
		},
		{
			name:   "file only",
			module: PlaceholderDump{File: "out.log", Content: "value"},
		},
		{
			name:   "logger only",
			module: PlaceholderDump{LoggerSuffix: "trace", Content: "value"},
		},
		{
			name:   "file and logger",
			module: PlaceholderDump{File: "out.log", LoggerSuffix: "trace", Content: "value"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.module.Validate()
			if tt.wantErr == "" && err != nil {
				t.Fatalf("Validate() returned unexpected error: %v", err)
			}
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("Validate() error = nil, want %q", tt.wantErr)
				}
				if err.Error() != tt.wantErr {
					t.Fatalf("Validate() error = %q, want %q", err.Error(), tt.wantErr)
				}
			}
		})
	}
}

func TestParseFilePerm(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input   string
		want    os.FileMode
		wantErr bool
	}{
		{input: "644", want: 0o644},
		{input: "0600", want: 0o600},
		{input: "0", want: 0},
		{input: "abc", wantErr: true},
		{input: "888", wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			got, err := parseFilePerm(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseFilePerm(%q) error = nil, want error", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseFilePerm(%q) returned error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Fatalf("parseFilePerm(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestPlaceholderDumpProvision(t *testing.T) {
	t.Parallel()

	existingMutex := &sync.Mutex{}

	tests := []struct {
		name                string
		module              *PlaceholderDump
		wantFilePermissions string
		wantParsedPerm      os.FileMode
		wantMutex           *sync.Mutex
	}{
		{
			name: "sets defaults",
			module: &PlaceholderDump{
				Content: "value",
			},
			wantFilePermissions: "644",
			wantParsedPerm:      0o644,
		},
		{
			name: "uses valid custom permissions and preserves mutex",
			module: &PlaceholderDump{
				FilePermissions: "600",
				mutex:           existingMutex,
			},
			wantFilePermissions: "600",
			wantParsedPerm:      0o600,
			wantMutex:           existingMutex,
		},
		{
			name: "falls back on invalid permissions",
			module: &PlaceholderDump{
				FilePermissions: "invalid",
			},
			wantFilePermissions: "invalid",
			wantParsedPerm:      0o644,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if err := tt.module.Provision(caddy.Context{}); err != nil {
				t.Fatalf("Provision() returned error: %v", err)
			}
			if tt.module.logger == nil {
				t.Fatal("Provision() did not initialize logger")
			}
			if tt.module.FilePermissions != tt.wantFilePermissions {
				t.Fatalf("FilePermissions = %q, want %q", tt.module.FilePermissions, tt.wantFilePermissions)
			}
			if tt.module.parsedFilePerm != tt.wantParsedPerm {
				t.Fatalf("parsedFilePerm = %v, want %v", tt.module.parsedFilePerm, tt.wantParsedPerm)
			}

			wantMutex := tt.wantMutex
			if wantMutex == nil {
				if tt.module.mutex == nil {
					t.Fatal("Provision() did not initialize mutex")
				}
			} else if tt.module.mutex != wantMutex {
				t.Fatalf("mutex = %p, want %p", tt.module.mutex, wantMutex)
			}
		})
	}
}

func TestPlaceholderDumpServeHTTPWritesResolvedContent(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	repl := caddy.NewReplacer()
	repl.Set("test.file", "trace")
	repl.Set("test.value", "hello world")

	module := &PlaceholderDump{
		File:           filepath.Join(tempDir, "{test.file}.log"),
		Content:        "  {test.value}  ",
		parsedFilePerm: 0o600,
		logger:         zap.NewNop(),
		mutex:          &sync.Mutex{},
	}

	next := &testNextHandler{}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), caddy.ReplacerCtxKey, repl))

	if err := module.ServeHTTP(httptest.NewRecorder(), req, next); err != nil {
		t.Fatalf("ServeHTTP() returned error: %v", err)
	}
	if next.calls != 1 {
		t.Fatalf("next calls = %d, want 1", next.calls)
	}

	content, err := os.ReadFile(filepath.Join(tempDir, "trace.log"))
	if err != nil {
		t.Fatalf("ReadFile() returned error: %v", err)
	}
	if string(content) != "hello world\n" {
		t.Fatalf("file content = %q, want %q", string(content), "hello world\n")
	}
}

func TestPlaceholderDumpServeHTTPLogsResolvedContent(t *testing.T) {
	t.Parallel()

	core, observedLogs := observer.New(zapcore.InfoLevel)
	repl := caddy.NewReplacer()
	repl.Set("test.logger", "audit")
	repl.Set("test.value", "hello world")

	module := &PlaceholderDump{
		LoggerSuffix: "{test.logger}",
		Content:      "  {test.value}  ",
		logger:       zap.New(core),
		mutex:        &sync.Mutex{},
	}

	next := &testNextHandler{}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), caddy.ReplacerCtxKey, repl))

	if err := module.ServeHTTP(httptest.NewRecorder(), req, next); err != nil {
		t.Fatalf("ServeHTTP() returned error: %v", err)
	}
	if next.calls != 1 {
		t.Fatalf("next calls = %d, want 1", next.calls)
	}

	entries := observedLogs.AllUntimed()
	if len(entries) != 1 {
		t.Fatalf("log entries = %d, want 1", len(entries))
	}

	entry := entries[0]
	if entry.LoggerName != "audit" {
		t.Fatalf("LoggerName = %q, want %q", entry.LoggerName, "audit")
	}
	if entry.Message != "Logging resolved content" {
		t.Fatalf("Message = %q, want %q", entry.Message, "Logging resolved content")
	}

	contextMap := entry.ContextMap()
	if got := contextMap["content"]; got != "hello world" {
		t.Fatalf("logged content = %#v, want %q", got, "hello world")
	}
}

func TestPlaceholderDumpServeHTTPSkipsEmptyResolvedContent(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	repl := caddy.NewReplacer()
	repl.Set("test.empty", "")

	module := &PlaceholderDump{
		File:           filepath.Join(tempDir, "trace.log"),
		Content:        "  {test.empty}  ",
		parsedFilePerm: 0o644,
		logger:         zap.NewNop(),
		mutex:          &sync.Mutex{},
	}

	next := &testNextHandler{}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), caddy.ReplacerCtxKey, repl))

	if err := module.ServeHTTP(httptest.NewRecorder(), req, next); err != nil {
		t.Fatalf("ServeHTTP() returned error: %v", err)
	}
	if next.calls != 1 {
		t.Fatalf("next calls = %d, want 1", next.calls)
	}

	if _, err := os.Stat(filepath.Join(tempDir, "trace.log")); !os.IsNotExist(err) {
		t.Fatalf("trace.log exists or returned unexpected error: %v", err)
	}
}

func TestPlaceholderDumpServeHTTPWithoutReplacer(t *testing.T) {
	t.Parallel()

	module := &PlaceholderDump{
		Content: "value",
		File:    "out.log",
		logger:  zap.NewNop(),
		mutex:   &sync.Mutex{},
	}

	next := &testNextHandler{}
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	err := module.ServeHTTP(httptest.NewRecorder(), req, next)
	if err == nil {
		t.Fatal("ServeHTTP() error = nil, want error")
	}

	handlerErr, ok := err.(caddyhttp.HandlerError)
	if !ok {
		t.Fatalf("ServeHTTP() error type = %T, want caddyhttp.HandlerError", err)
	}
	if handlerErr.StatusCode != http.StatusInternalServerError {
		t.Fatalf("StatusCode = %d, want %d", handlerErr.StatusCode, http.StatusInternalServerError)
	}
	if next.calls != 0 {
		t.Fatalf("next calls = %d, want 0", next.calls)
	}
}

func TestPlaceholderDumpServeHTTPContinuesOnFileOpenError(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	repl := caddy.NewReplacer()
	repl.Set("test.value", "hello world")

	module := &PlaceholderDump{
		File:           filepath.Join(tempDir, "missing", "trace.log"),
		Content:        "{test.value}",
		parsedFilePerm: 0o644,
		logger:         zap.NewNop(),
		mutex:          &sync.Mutex{},
	}

	next := &testNextHandler{}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), caddy.ReplacerCtxKey, repl))

	if err := module.ServeHTTP(httptest.NewRecorder(), req, next); err != nil {
		t.Fatalf("ServeHTTP() returned error: %v", err)
	}
	if next.calls != 1 {
		t.Fatalf("next calls = %d, want 1", next.calls)
	}
}
