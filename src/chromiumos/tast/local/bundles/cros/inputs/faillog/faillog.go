// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package faillog provides helper functions for dumping UI data on test failures.
package faillog

import (
	"context"
	"os"
	"path/filepath"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/testing"
)

const (
	faillogDir     = "faillog"
	uiTreeFileName = "ui_tree.txt"
)

// DumpUITreeOnError checks the testing.State and dumps the whole UI tree data
// into a file 'ui_tree.txt' when the test fails. It does nothing when the test
// succeeds.
func DumpUITreeOnError(ctx context.Context, s *testing.State, tconn *chrome.TestConn) {
	if !s.HasError() {
		return
	}

	dir := filepath.Join(s.OutDir(), faillogDir)
	if err := os.MkdirAll(dir, 0777); err != nil {
		s.Logf("Failed to create directory %s: %v", dir, err)
		return
	}

	fileName := filepath.Join(dir, uiTreeFileName)
	s.Log("Test failed. Dumping the automation node tree into ", fileName)
	if err := ui.LogRootDebugInfo(ctx, tconn, fileName); err != nil {
		s.Log("Failed to dump: ", err)
	}
}
