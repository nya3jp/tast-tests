// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         GCA,
		Desc:         "Test GCA",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "chrome_login"},
		Timeout:      5 * time.Minute,
	})
}

func GCA(ctx context.Context, s *testing.State) {
	const (
		pkg    = "com.google.android.GoogleCameraArc"
		intent = "com.android.camera.CameraLauncher"

		appRootViewID   = "com.google.android.GoogleCameraArc:id/activity_root_view"
		shutterButtonID = "com.google.android.GoogleCameraArc:id/shutter_button"
	)

	cr, err := chrome.New(ctx, chrome.ARCEnabled())
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

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

	s.Log("Granting location permission to GCA")
	a.Command(ctx, "pm", "grant", pkg, "android.permission.ACCESS_FINE_LOCATION").Run()
	a.Command(ctx, "pm", "grant", pkg, "android.permission.ACCESS_COARSE_LOCATION").Run()

	s.Log("Starting app")
	if err := a.Command(ctx, "am", "start", "-W", "-n", pkg+"/"+intent).Run(); err != nil {
		s.Fatal("Failed starting app: ", err)
	}

	must := func(err error) {
		if err != nil {
			s.Fatal(err)
		}
	}

	// Wait for the UI to inflate
	must(d.Object(ui.ID(appRootViewID)).WaitForExists(ctx))

	// Click the shutter button.
	s.Log("Clicking the shutter button")
	must(d.Object(ui.ID(shutterButtonID)).Click(ctx))
}
