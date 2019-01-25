// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         GoogleCameraArc,
		Desc:         "Verify GoogleCameraArc can be launched successfully",
		Contacts:     []string{"lnishan@chromium.org"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "chrome_login"},
		Timeout:      5 * time.Minute,
	})
}

func GoogleCameraArc(ctx context.Context, s *testing.State) {
	const (
		pkg    = "com.google.android.GoogleCameraArc"
		intent = "com.android.camera.CameraLauncher"

		appRootViewID = "com.google.android.GoogleCameraArc:id/activity_root_view"
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

	s.Log("Granting location permission to GCA")
	a.Command(ctx, "pm", "grant", pkg, "android.permission.ACCESS_FINE_LOCATION").Run()
	a.Command(ctx, "pm", "grant", pkg, "android.permission.ACCESS_COARSE_LOCATION").Run()

	s.Log("Starting app")
	if err := a.Command(ctx, "am", "start", "-W", "-n", pkg+"/"+intent).Run(); err != nil {
		s.Fatal("Failed starting app: ", err)
	}

	s.Log("Waiting 3 seconds for the app to load up")
	time.After(3 * time.Second)

	var pid []byte
	pid, err = a.Command(ctx, "pidof", pkg).Output()
	s.Log("pid of " + pkg + " = " + string(pid))
	if len(pid) == 0 {
		s.Fatal("App crashed during start-up")
	}
}
