// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package faillog contains a method for dumping useful information on error
package faillog

import (
	"context"
	"os"
	"path"

	"chromiumos/tast/local/chrome"
	uifaillog "chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/faillog"
	"chromiumos/tast/testing"
)

// DumpUITreeAndScreenshot records the current UI tree in
// "faillog/${name}_ui_tree.txt" in the out directory and the standard test
// failure data (process list, upstart jobs, and screenshot) in the
// directory "${name}_faillog".
//
// |name| is used as a disambiguator to prevent multiple different
// calls from writing to the same place and should be unique per
// callsite.
//
// |err| is used to control if anything is actually done. If err is
// nil, nothing is recorded. The intended usage is to do something
// like
// defer func() { faillog.DumpUITreeAndScreenshot(ctx, tconn, "crostini_installer", retErr) }()
// in suspect functions, where |err| is a named return variable to
// cause logs to be recorded only when the function returns an
// error. Note that the wrapping closure is required to delay
// evaluating |retErr| until return time.
func DumpUITreeAndScreenshot(ctx context.Context, tconn *chrome.TestConn, name string, err interface{}) {
	testing.ContextLog(ctx, "Dumping UI tree")
	if err != nil {
		testing.ContextLog(ctx, "err != nil")
		outDir, ok := testing.ContextOutDir(ctx)
		if !ok {
			testing.ContextLog(ctx, "Failed to get out directory: ", err)
		}

		uifaillog.DumpUITreeOnErrorToFile(ctx, outDir, func() bool { return true }, tconn, name+"_ui_tree.txt")

		subDir := path.Join(outDir, name+"_faillog")
		if err := os.MkdirAll(subDir, 0755); err != nil {
			testing.ContextLog(ctx, "Failed to make error directory: ", err)
		}
		faillog.SaveToDir(ctx, subDir)
	}
}
