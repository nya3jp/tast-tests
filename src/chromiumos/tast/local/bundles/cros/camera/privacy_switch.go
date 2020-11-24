// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"fmt"
	"regexp"

	"chromiumos/tast/local/bundles/cros/camera/testutil"
	"chromiumos/tast/local/crosconfig"
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
		SoftwareDeps: []string{caps.BuiltinUSBCamera},
	})
}

var ctrlBusy = regexp.MustCompile(`(?m)^VIDIOC_G_EXT_CTRLS: failed: Device or resource busy$`)

func hasPrivacySwitchControl(ctx context.Context, s *testing.State) bool {

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
			s.Logf("Device %s has a Privacy Control", devicePath)
			return true
		}

		if cmd.ProcessState.ExitCode() == 255 {
			if ctrlBusy.Match(out) {
				s.Logf("Device %s has a Privacy Control", devicePath)
				return true
			}
		}
	}

	s.Log("No Device has a Privacy Control")
	return false
}

func hasPrivacySwitchHardware(ctx context.Context, s *testing.State) bool {

	for i := 0; ; i++ {
		val, err := crosconfig.Get(ctx, fmt.Sprintf("/camera/devices/%v", i), "has-privacy-switch")
		if err != nil {
			if crosconfig.IsNotFound(err) {
				break
			}
			s.Fatal("Failed to execute cros_config: ", err)
		}
		if val == "true" {
			s.Logf("Camera %v supports Privacy Switch", i)
			return true
		}
	}
	s.Log("No Privacy Switch Found")
	return false
}

func PrivacySwitch(ctx context.Context, s *testing.State) {

	hasControl := hasPrivacySwitchControl(ctx, s)
	hasHardware := hasPrivacySwitchHardware(ctx, s)

	if hasHardware && !hasControl {
		s.Error("PrivacySwitch present but no video device can access it")
	}

	if hasControl && !hasHardware {
		s.Error("PrivacySwitch not present in hardware but accessible via v4l control")
	}

}
