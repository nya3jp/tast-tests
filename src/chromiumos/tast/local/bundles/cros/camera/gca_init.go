// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         GCAInit,
		Desc:         "Verify GoogleCameraArc can be launched successfully",
		Contacts:     []string{"lnishan@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "chrome_login"},
		Timeout:      4 * time.Minute,
	})
}

func GCAInit(ctx context.Context, s *testing.State) {
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

	// GCA would ask for location permission during startup. We need to dismiss
	// the dialog before we can use the app.
	s.Log("Granting all needed permissions (e.g., location) to GCA")
	if err := a.Command(ctx, "pm", "grant", pkg, "android.permission.ACCESS_FINE_LOCATION").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to grant ACCESS_FINE_LOCATION permission to GCA")
	}
	if err := a.Command(ctx, "pm", "grant", pkg, "android.permission.ACCESS_COARSE_LOCATION").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to grant ACCESS_COARSE_LOCATION permission to GCA")
	}

	s.Log("Starting app")
	if err := a.Command(ctx, "am", "start", "-W", "-n", pkg+"/"+intent).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed starting app: ", err)
	}
	err = testing.Poll(ctx, func(ctx context.Context) error {
		var pid []byte
		if pid, err = a.Command(ctx, "pidof", pkg).Output(testexec.DumpLogOnError); err != nil {
			s.Fatal("Failed to query pid of GCA")
		}
		if len(pid) == 0 {
			return errors.New("Process not yet started or app crashed")
		}
		s.Logf("Successfully queried pid of %s: %s", pkg, string(pid))
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second})
	if err != nil {
		s.Fatal("Unable to query pid of GCA after 10 seconds, assuming app crashed")
	}
}
