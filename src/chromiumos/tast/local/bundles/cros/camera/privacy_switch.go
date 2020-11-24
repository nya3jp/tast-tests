// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"regexp"

	"chromiumos/tast/local/bundles/cros/camera/testutil"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: PrivacySwitch,
		Desc: "Verifies the privacy switch",
		Contacts: []string{
			"ribalda@chromium.org",
			"chromeos-camera-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{caps.BuiltinUSBCamera, caps.CameraPrivacySwitch},
	})
}

var ctrlBusy = regexp.MustCompile(`(?m)^VIDIOC_G_EXT_CTRLS: failed: Device or resource busy$`)

func PrivacySwitch(ctx context.Context, s *testing.State) {

	usbCams, err := testutil.GetUSBCamerasFromV4L2Test(ctx)
	if err != nil {
		s.Fatal("Failed to get USB cameras: ", err)
	}
	if len(usbCams) == 0 {
		s.Fatal("Failed to find any valid device")
	}
	s.Log("USB cameras: ", usbCams)

	for _, devicePath := range usbCams {

		cmd := testexec.CommandContext(ctx, "v4l2-ctl", "-C", "privacy", "-d", devicePath)
		out, err := cmd.Output(testexec.DumpLogOnError)

		if err == nil {
			continue
		}

		if cmd.ProcessState.ExitCode() == 255 {
			if ctrlBusy.Match(out) {
				continue
			}
		}

		s.Error("Error accessing Privacy Control on ", devicePath, " ", err, " ", string(out))
	}

}
