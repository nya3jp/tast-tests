// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"fmt"
	"regexp"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/camera/testutil"
	"chromiumos/tast/local/crosconfig"
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

func hasPrivacySwitchControl(ctx context.Context) (bool, error) {

	usbCams, err := testutil.GetUSBCamerasFromV4L2Test(ctx)
	if err != nil {
		return false, errors.Wrap(err, "failed to get USB cameras")
	}
	if len(usbCams) == 0 {
		return false, errors.New("failed to find any valid device")
	}
	testing.ContextLog(ctx, "USB cameras: ", usbCams)

	for _, devicePath := range usbCams {

		cmd := testexec.CommandContext(ctx, "v4l2-ctl", "-C", "privacy", "-d", devicePath)
		out, err := cmd.Output(testexec.DumpLogOnError)

		if err == nil || (cmd.ProcessState.ExitCode() == 255 && ctrlBusy.Match(out)) {
			testing.ContextLogf(ctx, "Device %s has a privacy control", devicePath)
			return true, nil
		}

		// An error != 255 indicates that the control does not exist, which is a valid result
	}

	testing.ContextLog(ctx, "No device has a privacy control")
	return false, nil
}

func hasPrivacySwitchHardware(ctx context.Context) (bool, error) {

	for i := 0; ; i++ {
		val, err := crosconfig.Get(ctx, fmt.Sprintf("/camera/devices/%v", i), "has-privacy-switch")
		if crosconfig.IsNotFound(err) {
			break
		}
		if err != nil {
			return false, errors.Wrap(err, "failed to execute cros_config")
		}
		if val == "true" {
			testing.ContextLogf(ctx, "Camera %v supports privacy switch", i)
			return true, nil
		}
	}
	testing.ContextLog(ctx, "No privacy switch found")
	return false, nil
}

func PrivacySwitch(ctx context.Context, s *testing.State) {

	hasControl, err := hasPrivacySwitchControl(ctx)
	if err != nil {
		s.Fatal("Failed to get privacy switch control: ", err)
	}
	hasHardware, err := hasPrivacySwitchHardware(ctx)
	if err != nil {
		s.Fatal("Failed to get privacy switch hardware: ", err)
	}

	if hasHardware && !hasControl {
		s.Error("Privacy switch present but no video device can access it")
	}

	if hasControl && !hasHardware {
		s.Error("Privacy switch not present in hardware but accessible via v4l control")
	}

}
