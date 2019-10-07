// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CameraOrientation,
		Desc: "Verifies camera orientation compatibility solution works",
		Contacts: []string{
			"shik@chromium.org",
			"chromeos-camera-eng@google.com",
			"hidehiko@chromium.org", // Tast port author.
		},
		Attr:         []string{"informational"},
		Data:         []string{"ArcCameraOrientationTest.apk"},
		SoftwareDeps: []string{"android", "chrome", caps.BuiltinUSBCamera},
		Pre:          arc.Booted(),
	})
}

func CameraOrientation(ctx context.Context, s *testing.State) {
	a := s.PreValue().(arc.PreData).ARC

	const (
		apk      = "ArcCameraOrientationTest.apk"
		pkg      = "org.chromium.arc.testapp.cameraorientation"
		activity = pkg + "/.MainActivity"

		// UI IDs in the app.
		idPrefix = pkg + ":id/"
		startID  = idPrefix + "start_test"
		resultID = idPrefix + "test_result"
		logID    = idPrefix + "test_result_log"
	)

	s.Logf("Installing %s", apk)
	if err := a.Install(ctx, s.DataPath(apk)); err != nil {
		s.Fatalf("Failed to install %s: %v", apk, err)
	}

	s.Log("Granting permission")
	if err := a.Command(ctx, "pm", "grant", pkg, "android.permission.CAMERA").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to grant CAMERA permission: ", err)
	}

	s.Log("Launching app")
	if err := a.Command(ctx, "am", "start", "-W", activity).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to launch the app: ", err)
	}

	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed to initialize UI Automator: ", err)
	}
	defer d.Close()

	if err := d.Object(ui.ID(startID)).Click(ctx); err != nil {
		s.Fatal("Failed to start testing: ", err)
	}

	if err := d.Object(ui.ID(resultID), ui.TextMatches("[01]")).WaitForExists(ctx, 20*time.Second); err != nil {
		s.Fatal("Timed out for waiting result updated: ", err)
	}

	// Test result can be either '0' or '1', where '0' means fail and '1'
	// means pass.
	if result, err := d.Object(ui.ID(resultID)).GetText(ctx); err != nil {
		s.Fatal("Failed to get the result: ", err)
	} else if result != "1" {
		// Note: failure reason reported from the app is one line,
		// so directly print it here.
		reason, err := d.Object(ui.ID(logID)).GetText(ctx)
		if err != nil {
			s.Fatal("Failed to get failure reason: ", err)
		}
		s.Error("Test failed: ", reason)
	}
}
