// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package apptest contains common utilities to help writing ARC app tests.
package apptest

import (
	"context"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

type testFunc func(a *arc.ARC, d *ui.Device)

// Run starts Chrome and then calls RunWithChrome.
func Run(ctx context.Context, s *testing.State, apk, pkg, cls string, f testFunc) {
	cr, err := chrome.New(ctx, chrome.ARCEnabled())
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	RunWithChrome(ctx, s, cr, apk, pkg, cls, f)
}

// RunWithChrome starts ARC in an existing Chrome instance. It then installs an app
// APK, sets up UI Automator and runs a test body function f.
// apk is a filename of an APK file in data directory.
// pkg/cls are package name and activity class name of the app to launch.
func RunWithChrome(ctx context.Context, s *testing.State, cr *chrome.Chrome, apk, pkg, cls string, f testFunc) {
	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close()

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
