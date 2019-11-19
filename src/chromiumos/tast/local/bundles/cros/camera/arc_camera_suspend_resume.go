// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ArcCameraSuspendResume,
		Desc:         "Ensures that camera orientation compatibility solution works as expected",
		Contacts:     []string{"henryhsu@chromium.org", "lnishan@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android", "chrome", caps.BuiltinCamera},
		Data:         []string{"ArcCameraSuspendResumeTest.apk"},
		Pre:          arc.Booted(),
		Timeout:      10 * time.Minute,
	})
}

func ArcCameraSuspendResume(ctx context.Context, s *testing.State) {
	const (
		apk = "ArcCameraSuspendResumeTest.apk"
		pkg = "org.chromium.arc.testapp.camerasuspendresume"
		act = pkg + "/.MainActivity"

		testResID    = pkg + ":id/test_result"
		testResLogID = pkg + ":id/test_result_log"
		duration     = 480 * time.Second
		maxResume    = 3 // seconds
		minSuspend   = 3 // seconds
	)

	endTime := time.Now().Add(duration)

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
	defer func() {
		// Close test app.
		if err := a.Command(ctx, "am", "force-stop", pkg).Run(testexec.DumpLogOnError); err != nil {
			s.Fatal("Failed to close test app: ", err)
		}
	}()

	s.Log("Waiting for camera to be opened")
	if err := d.Object(ui.ID(testResID), ui.TextContains("1")).WaitForExists(ctx, 20*time.Second); err != nil {
		s.Fatal("Failed to wait for camera to be opened: ", err)
	}

	iter := 0
	for endTime.Sub(time.Now()).Seconds() > 0 {
		iter++

		if err := d.Object(ui.ID(testResID)).SetText(ctx, ""); err != nil {
			s.Fatal("Failed to clear the text in the result field: ", err)
		}

		testing.Sleep(ctx, time.Duration(rand.Intn(maxResume+1))*time.Second)

		suspendTime := minSuspend + rand.Intn(4)
		s.Logf("Suspending for %d seconds", suspendTime)
		suspendArg := fmt.Sprintf("--suspend_for_sec=%d", suspendTime)

		if err := testexec.CommandContext(ctx, "powerd_dbus_suspend", suspendArg).Run(testexec.DumpLogOnError); err != nil {
			s.Fatal("Failed to suspend the system: ", err)
		}

		testing.Sleep(ctx, 20*time.Second)

		if err := d.Object(ui.ID(testResID), ui.TextMatches("[01]")).WaitForExists(ctx, 30*time.Second); err != nil {
			s.Fatalf("Test failed (%d): open camera failed", iter)
		}

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
			s.Fatalf("Test failed (%d): [%s] %s", iter, res, log)
		} else {
			s.Logf("Suspend and resume %d times success", iter)
		}
	}
}
