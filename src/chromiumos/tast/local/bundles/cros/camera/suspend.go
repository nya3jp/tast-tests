// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"time"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/camera/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Suspend,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies the camera stack works after a suspend",
		Contacts: []string{
			"chromeos-camera-eng@google.com",
			"ribalda@chromium.org",
		},
		BugComponent: "b:167281",
		Attr:         []string{"group:mainline", "informational", "group:camera-libcamera"},
		SoftwareDeps: []string{"arc_camera3", "chrome", caps.BuiltinCamera},
	})
}

func Suspend(ctx context.Context, s *testing.State) {
	// TODO(b/151270948): Temporarily disable ARC.
	// The cros-camera service would kill itself when running the test if
	// arc_setup.cc is triggered at that time, which will fail the test.
	cr, err := chrome.New(ctx, chrome.ARCDisabled(), chrome.NoLogin())
	if err != nil {
		s.Fatal("Failed to start chrome: ", err)
	}
	defer cr.Close(ctx)

	// Leave some time for cr.Close.
	ctx, cancel := ctxutil.Shorten(ctx, time.Second)
	defer cancel()

	err = testutil.WaitForCameraSocket(ctx)
	if err != nil {
		s.Fatal("Failed to wait for Camera Socket: ", err)
	}

	cmd := testexec.CommandContext(ctx, "cros_camera_connector_test", "--gtest_filter=ConnectorTest/CaptureTest.OneFrame/NV12_640x480_30fps")
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to use camera before suspend: ", err)
	}

	cmd = testexec.CommandContext(ctx, "powerd_dbus_suspend", "--delay=0", "--suspend_for_sec=5", "--wakeup_timeout=10")
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to suspend: ", err)
	}

	err = testutil.WaitForCameraSocket(ctx)
	if err != nil {
		s.Fatal("Failed to wait for Camera Socket after suspend: ", err)
	}

	cmd = testexec.CommandContext(ctx, "cros_camera_connector_test", "--gtest_filter=ConnectorTest/CaptureTest.OneFrame/NV12_640x480_30fps")
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to use camera after suspend: ", err)
	}
}
