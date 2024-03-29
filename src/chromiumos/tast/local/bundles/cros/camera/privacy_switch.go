// Copyright 2021 The ChromiumOS Authors
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
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PrivacySwitch,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies the privacy switch",
		Contacts: []string{
			"ribalda@chromium.org",
			"chromeos-camera-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{caps.BuiltinUSBCamera},
		// Primus camera module's privacy switch is not connected to the shutter b/236661871
		HardwareDeps: hwdep.D(hwdep.SkipOnModel("primus")),
	})
}

var ctrlBusy = regexp.MustCompile(`(?m)^VIDIOC_G_EXT_CTRLS: failed: Device or resource busy$`)

func hasPrivacySwitchControl(ctx context.Context) (bool, error) {

	usbCams, err := testutil.USBCamerasFromV4L2Test(ctx)
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

type privacySwitchPresence uint

const (
	privacySwitchNotPresent privacySwitchPresence = iota
	privacySwitchPresent
	privacySwitchIgnore
)

func hasPrivacySwitchHardware(ctx context.Context) (privacySwitchPresence, error) {

	for i := 0; ; i++ {
		device := fmt.Sprintf("/camera/devices/%v", i)
		_, err := crosconfig.Get(ctx, device, "interface")
		if crosconfig.IsNotFound(err) {
			break
		}
		if err != nil {
			return privacySwitchNotPresent, errors.Wrap(err, "failed to execute cros_config")
		}
		val, err := crosconfig.Get(ctx, device, "has-privacy-switch")
		if crosconfig.IsNotFound(err) {
			continue
		}
		if err != nil {
			return privacySwitchNotPresent, errors.Wrap(err, "failed to execute cros_config")
		}
		if val == "true" {
			testing.ContextLogf(ctx, "Camera %v supports privacy switch", i)
			return privacySwitchPresent, nil
		}
		if val == "false" {
			testing.ContextLogf(ctx, "Camera %v has unconnected privacy switch", i)
			return privacySwitchIgnore, nil
		}
	}
	testing.ContextLog(ctx, "No privacy switch found")
	return privacySwitchNotPresent, nil
}

func PrivacySwitch(ctx context.Context, s *testing.State) {

	hasControl, err := hasPrivacySwitchControl(ctx)
	if err != nil {
		s.Fatal("Failed to get privacy switch control: ", err)
	}
	privacySwitch, err := hasPrivacySwitchHardware(ctx)
	if err != nil {
		s.Fatal("Failed to get privacy switch hardware: ", err)
	}

	if privacySwitch == privacySwitchPresent && !hasControl {
		s.Error("Privacy switch present but no video device can access it")
	}

	if privacySwitch == privacySwitchNotPresent && hasControl {
		s.Error("Privacy switch not present in hardware but accessible via v4l control")
	}

}
