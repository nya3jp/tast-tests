// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package faillog can be used to save "faillog" on test failures. A faillog is
// a collection of log files which can be used to debug test failures.
package faillog

import (
	"context"
	"os"
	"path/filepath"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// SaveIfError saves a faillog only if the test has any errors.
func SaveIfError(s *testing.State) {
	if s.HasError() {
		Save(s)
	}
}

// Save saves a faillog unconditionally.
func Save(s *testing.State) {
	ctx := s.Context()

	dir := filepath.Join(s.OutDir(), "faillog")
	if err := os.MkdirAll(dir, 0755); err != nil {
		s.Logf("Failed creating %s: %v", dir, err)
		return
	}

	savePs(ctx, dir)
	saveScreenshot(ctx, dir)
}

// savePs saves "ps" output.
func savePs(ctx context.Context, dir string) {
	path := filepath.Join(dir, "ps.txt")
	f, err := os.Create(path)
	if err != nil {
		testing.ContextLogf(ctx, "Failed creating %s: %v", path, err)
		return
	}
	defer f.Close()

	cmd := testexec.CommandContext(context.Background(), "ps", "auxwwf")
	cmd.Stdout = f
	if err := cmd.Run(); err != nil {
		cmd.DumpLog(ctx)
		testing.ContextLog(ctx, "Failed saving ps: ", err)
	}
}

// saveScreenshot saves a screenshot.
func saveScreenshot(ctx context.Context, dir string) {
	path := filepath.Join(dir, "screenshot.png")
	cmd := testexec.CommandContext(context.Background(), "screenshot", path)
	if err := cmd.Run(); err != nil {
		cmd.DumpLog(ctx)
		testing.ContextLog(ctx, "Failed saving a screenshot: ", err)
	}
}
