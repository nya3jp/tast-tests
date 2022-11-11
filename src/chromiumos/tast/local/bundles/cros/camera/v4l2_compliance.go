// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/camera/testutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: V4L2Compliance,
		Desc: "Runs V4L2Compliance in all the Capture Devices",
		Contacts: []string{
			"chromeos-camera-kernel@google.com",
			"chromeos-camera-eng@google.com",
			"ribalda@chromium.org",
		},
		BugComponent: "b:1093480",
		Attr:         []string{"group:mainline", "group:camera-usb-qual", "informational"},
	})
}

func V4L2Compliance(ctx context.Context, s *testing.State) {

	captureDevices, err := testutil.CaptureDevicesFromV4L2Test(ctx)
	if err != nil {
		s.Fatal("Failed to list Capture Devices: ", err)
	}

	for _, videodev := range captureDevices {
		cmd := testexec.CommandContext(ctx, "v4l2-compliance", "-v", "-d", videodev)
		out, err := cmd.Output(testexec.DumpLogOnError)

		if err == nil {
			continue
		}

		// Log full output on error.
		result := string(out)
		s.Log(result)

		if cmd.ProcessState.ExitCode() != 1 {
			s.Fatalf("v4l2-compliance failed: %s: %v", videodev, err)
		}

		// Remove last end of line if present.
		result = strings.TrimSuffix(result, "\n")
		// Get last line.
		lastline := result[strings.LastIndex(result, "\n"):]
		s.Errorf("v4l2-compliance failed: %s: %s", videodev, lastline)
	}
}
