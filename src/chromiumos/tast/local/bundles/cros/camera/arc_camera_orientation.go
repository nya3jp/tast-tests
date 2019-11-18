// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

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
		Func:         ArcCameraOrientation,
		Desc:         "Ensures that camera orientation compatibility solution works as expected",
		Contacts:     []string{"lnishan@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android", "chrome", caps.BuiltinCamera},
		Data:         []string{"ArcCameraOrientationTest.apk"},
		Pre:          arc.Booted(),
		Timeout:      4 * time.Minute,
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

	a := s.PreValue().(arc.PreData).ARC
	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close()

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
			s.Fatal(err) // NOLINT: arc/ui returns loggable errors
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
