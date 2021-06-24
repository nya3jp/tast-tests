// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"os"
	"path/filepath"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/android/ui"
)

// NewUIDevice creates a Device object by starting and connecting to UI Automator server.
// Close must be called to clean up resources when a test is over.
func (a *ARC) NewUIDevice(ctx context.Context) (*ui.Device, error) {
	return ui.NewDevice(ctx, a.device)
}

// DumpUIHierarchyOnError dumps arc UI hierarchy to 'arc_uidump.xml', when the test fails.
// Call this function after closing arc UI devices. Otherwise the uiautomator might exist with errors like
// status 137.
func (a *ARC) DumpUIHierarchyOnError(ctx context.Context, outDir string, hasError func() bool) error {
	if !hasError() {
		return nil
	}

	if err := a.Command(ctx, "uiautomator", "dump").Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to dump arc UI")
	}

	dir := filepath.Join(outDir, "faillog")
	if err := os.MkdirAll(dir, 0777); err != nil {
		return errors.Wrapf(err, "failed to create directory %s", dir)
	}

	file := filepath.Join(dir, "arc_uidump.xml")
	if err := a.PullFile(ctx, "/sdcard/window_dump.xml", file); err != nil {
		return errors.Wrap(err, "failed to pull UI dump to outDir")
	}

	return nil
}
