// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package memoryuser

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/testing"
)

// AndroidTask implements MemoryTask to run ARC apps.
type AndroidTask struct {
	// ApkPath is the path to use to access the apk file
	ApkPath string
	// Apk is a filename of an APK file in the data directory.
	Apk string
	// Pkg is the package name of the app to launch.
	Pkg string
	// ActivityName is the activity class name of the app to launch.
	ActivityName string
	// TestFunc is the test body function to run.
	TestFunc func(a *arc.ARC, d *ui.Device)
}

// Run installs the app apk and runs the test function defined in the AndroidTask in the existing ARC instance.
func (at *AndroidTask) Run(ctx context.Context, testEnv *TestEnv) error {
	testing.ContextLog(ctx, "Starting app")
	if err := testEnv.ARC.Install(ctx, at.ApkPath); err != nil {
		return errors.Wrapf(err, "failed installing app %s", at.Apk)
	}

	if err := testEnv.ARC.Command(ctx, "am", "start", "-W", at.Pkg+"/"+at.ActivityName).Run(); err != nil {
		return errors.Wrapf(err, "failed starting app %s", at.Apk)
	}

	at.TestFunc(testEnv.ARC, testEnv.ARCDevice)
	return nil
}

// Close stops everything associated with the package defined in the AndroidTask
func (at *AndroidTask) Close(ctx context.Context, testEnv *TestEnv) {
	testEnv.ARC.Command(ctx, "am", "force-stop", at.Pkg).Run()
}
