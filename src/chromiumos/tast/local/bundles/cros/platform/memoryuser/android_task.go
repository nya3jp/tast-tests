// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package memoryuser

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/testing"
)

// AndroidTask implements MemoryTask to run ARC apps.
type AndroidTask struct {
	// APKPath is the path to use to access the APK file.
	APKPath string
	// APK is a filename of an APK file in the data directory.
	APK string
	// Pkg is the package name of the app to launch.
	Pkg string
	// ActivityName is the activity class name of the app to launch.
	ActivityName string
	// TestFunc is the test body function to run.
	TestFunc func(a *arc.ARC)
}

// Run installs the app APK and runs the test function defined in the AndroidTask in the existing ARC instance.
func (at *AndroidTask) Run(ctx context.Context, s *testing.State, testEnv *TestEnv) error {
	testing.ContextLog(ctx, "Starting app ", at.APK)
	startTime := time.Now()
	if err := testEnv.arc.Install(ctx, at.APKPath); err != nil {
		return errors.Wrapf(err, "failed installing app %s", at.APKPath)
	}

	if err := testEnv.arc.Command(ctx, "am", "start", "-W", at.Pkg+"/"+at.ActivityName).Run(); err != nil {
		return errors.Wrapf(err, "failed starting app %s", at.APK)
	}
	loadingTime := time.Now().Sub(startTime)
	testing.ContextLogf(ctx, "App install/start time for %s: %v", at.APK, loadingTime)
	at.TestFunc(testEnv.arc)
	return nil
}

// Close stops everything associated with the package defined in the AndroidTask.
func (at *AndroidTask) Close(ctx context.Context, testEnv *TestEnv) {
	testEnv.arc.Command(ctx, "am", "force-stop", at.Pkg).Run()
}

// String returns a string describing the AndroidTask.
func (at *AndroidTask) String() string {
	return fmt.Sprintf("AndroidTask with APK: %s, pkg: %s, activity: %s", at.APK, at.Pkg, at.ActivityName)
}

// NeedVM returns false to indicate no VM is required for an AndroidTask.
func (at *AndroidTask) NeedVM() bool {
	return false
}
