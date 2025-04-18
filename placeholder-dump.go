// Copyright 2025 Steffen Busch

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

// 	http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package placeholderdump

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"go.uber.org/zap"
)

// PlaceholderDump is a Caddy module that dumps a placeholder to a file.
// It logs the resolved placeholder values to the specified file.
type PlaceholderDump struct {
	// File is the path to the file where the content will be written.
	// If the file does not exist, it will be created.
	File string `json:"file,omitempty"`

	// Content is the content to be written to the file.
	// It can contain placeholders that will be resolved at runtime.
	Content string `json:"content,omitempty"`

	// logger provides structured logging for the module.
	// It's initialized in the Provision method and used throughout the module for debug information.
	logger *zap.Logger

	// mutex ensures thread-safe writes to the file for this instance.
	// However, if the file is shared across multiple instances, there is a risk of
	// concurrent writes leading to data corruption.
	// But this module is intended that each use of the module has its own file.
	mutex *sync.Mutex
}

// CaddyModule returns the Caddy module information.
func (PlaceholderDump) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.placeholder_dump",
		New: func() caddy.Module { return new(PlaceholderDump) },
	}
}

// Provision prepares the module for runtime execution by setting up the logger and initializing the mutex.
func (m *PlaceholderDump) Provision(ctx caddy.Context) error {
	m.logger = ctx.Logger(m)

	// Initialize the mutex if it's nil
	if m.mutex == nil {
		m.mutex = &sync.Mutex{}
	}
	return nil
}

// Validate ensures the configuration is correct.
func (m *PlaceholderDump) Validate() error {
	if m.File == "" {
		return fmt.Errorf("file must be set")
	}
	if m.Content == "" {
		return fmt.Errorf("content must be set")
	}
	return nil
}

// ServeHTTP handles incoming HTTP requests and writes the resolved content to the specified file.
func (m *PlaceholderDump) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	// Retrieve the replacer from the request context.
	repl, ok := r.Context().Value(caddy.ReplacerCtxKey).(*caddy.Replacer)
	if !ok {
		return caddyhttp.Error(http.StatusInternalServerError, nil)
	}

	// Resolve placeholders in the content.
	resolvedContent := repl.ReplaceAll(m.Content, "")
	resolvedContent = strings.TrimSpace(resolvedContent)

	// Skip writing if the resolved content is empty.
	if resolvedContent == "" {
		m.logger.Warn("Resolved content is empty; skipping write")
		return next.ServeHTTP(w, r)
	}

	// Lock the instance-specific mutex to ensure thread-safe file writes.
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Open the file for appending, creating it if it doesn't exist.
	const filePermissions = 0644
	f, err := os.OpenFile(m.File, os.O_APPEND|os.O_WRONLY|os.O_CREATE, filePermissions)
	if err != nil {
		m.logger.Error("Failed to open file", zap.String("file", m.File), zap.Error(err))
		return next.ServeHTTP(w, r)
	}
	defer f.Close()

	// Write the resolved content to the file.
	if _, err := f.WriteString(resolvedContent + "\n"); err != nil {
		m.logger.Error("Failed to write to file", zap.Error(err))
	} else {
		m.logger.Debug("Wrote content to file", zap.String("file", m.File), zap.String("content", resolvedContent))
	}

	return next.ServeHTTP(w, r)
}

// Interface guards
var (
	_ caddy.Module                = (*PlaceholderDump)(nil)
	_ caddy.Provisioner           = (*PlaceholderDump)(nil)
	_ caddy.Validator             = (*PlaceholderDump)(nil)
	_ caddyhttp.MiddlewareHandler = (*PlaceholderDump)(nil)
)
