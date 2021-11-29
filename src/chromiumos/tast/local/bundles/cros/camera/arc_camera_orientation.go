// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ArcCameraOrientation,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Ensures that camera orientation compatibility solution works as expected",
		Contacts:     []string{"shik@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:camera-libcamera"},
		SoftwareDeps: []string{"chrome", caps.BuiltinOrVividCamera},
		Pre:          arc.Booted(),
		Data:         []string{"ArcCameraOrientationTest.apk"},
		Timeout:      4 * time.Minute,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func ArcCameraOrientation(ctx context.Context, s *testing.State) {
	const (
		apk = "ArcCameraOrientationTest.apk"
		pkg = "org.chromium.arc.testapp.cameraorientation"
		act = pkg + "/.MainActivity"

		startTestID  = pkg + ":id/start_test"
		testResID    = pkg + ":id/test_result"
		testResLogID = pkg + ":id/test_result_log"
	)

	cr := s.PreValue().(arc.PreData).Chrome
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get Test API connection: ", err)
	}

	// The testing app expects to be launched in the clamshell mode.
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure in clamshell mode: ", err)
	}
	defer cleanup(ctx)

	a := s.PreValue().(arc.PreData).ARC
	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close(ctx)

	s.Log("Installing app and granting needed permission")
	if err := a.Install(ctx, s.DataPath(apk)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	if err := a.Command(ctx, "pm", "grant", pkg, "android.permission.CAMERA").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed granting camera permission to test app: ", err)
	}

	s.Log("Starting app")
	if err := a.Command(ctx, "am", "start", "-W", act).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed starting app: ", err)
	}

	must := func(err error) {
		if err != nil {
			s.Fatal(err) // NOLINT: adb/ui returns loggable errors
		}
	}

	// Wait until the current activity is idle.
	must(d.WaitForIdle(ctx, 10*time.Second))

	// Click the button which starts the test.
	must(d.Object(ui.ID(startTestID)).Click(ctx))

	// Wait for result.
	must(d.Object(ui.ID(testResID), ui.TextMatches("[01]")).WaitForExists(ctx, 20*time.Second))

	// Read result.
	res, err := d.Object(ui.ID(testResID)).GetText(ctx)
	if err != nil {
		s.Fatal("Failed to read test result: ", err)
	}

	// Read result log.
	log, err := d.Object(ui.ID(testResLogID)).GetText(ctx)
	if err != nil {
		s.Fatal("Failed to read test result log: ", err)
	}

	if res != "1" {
		s.Fatal("Test failed: ", log)
	}
}
