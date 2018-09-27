// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package apptest contains common utilities to help writing ARC app tests.
package apptest

import (
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/testing"
)

type testFunc func(a *arc.ARC, d *ui.Device)

// Run starts Chrome and ARC, installs an app APK, sets up UI Automator, and runs
// a test body function f.
// apk is a filename of an APK file in data directory.
// pkg/cls are package name and activity class name of the app to launch.
func Run(s *testing.State, a *arc.ARC, apk, pkg, cls string, f testFunc) {
	ctx := s.Context()
	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close()

	s.Log("Starting app")

	if err := a.Install(ctx, s.DataPath(apk)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	if err := a.Command(ctx, "am", "start", "-W", pkg+"/"+cls).Run(); err != nil {
		s.Fatal("Failed starting app: ", err)
	}

	f(a, d)
}
