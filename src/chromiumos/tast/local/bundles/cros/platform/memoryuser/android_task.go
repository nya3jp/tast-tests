// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package memoryuser contains common code to run multifaceted memory tests
// with Chrome, ARC, and VMs
package memoryuser

import (
	"context"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/testing"
)

// AndroidTask contains the apk, pkg, activity, and a test function for an ARC app.
// Apk is a filename of an APK file in the data directory.
// Pkg/ActivityName are the package and activity class name of the app to launch.
// TestFunc is a test body function to run.
type AndroidTask struct {
	Apk          string
	Pkg          string
	ActivityName string
	TestFunc     func(a *arc.ARC, d *ui.Device)
}

// RunMemoryTask installs the app apk and runs the test function defined in the AndroidTask in the existing ARC instance.
func (androidTask AndroidTask) RunMemoryTask(ctx context.Context, s *testing.State, testEnv *TestEnvironment) {
	s.Log("Starting app")

	if err := testEnv.Arc.Install(ctx, s.DataPath(androidTask.Apk)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	if err := testEnv.Arc.Command(ctx, "am", "start", "-W", androidTask.Pkg+"/"+androidTask.ActivityName).Run(); err != nil {
		s.Fatal("Failed starting app: ", err)
	}

	androidTask.TestFunc(testEnv.Arc, testEnv.ArcDevice)
}
