// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/remote/powercontrol"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/power"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type deviceFunctionality struct {
	functionality string
}

const (
	lidCloseOpenWithUsb2 string = "lidCloseOpenWithUsb2"
	systemIdleWithUsb2   string = "systemIdleWithUsb2"
	onlySystemIdle       string = "onlySystemIdle"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DeviceFunctionalityAfterSleep,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Device functionality after sleep (Keep system idle)/(Close lid)",
		HardwareDeps: hwdep.D(hwdep.X86()),
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.power.USBService"},
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		Fixture:      fixture.NormalMode,
		Params: []testing.Param{{
			Name:    "lid_close_open_with_usb2",
			Val:     deviceFunctionality{functionality: lidCloseOpenWithUsb2},
			Timeout: 10 * time.Minute,
		}, {
			Name:    "system_idle_with_usb2",
			Val:     deviceFunctionality{functionality: systemIdleWithUsb2},
			Timeout: 10 * time.Minute,
		}, {
			Name:    "only_system_idle",
			Val:     deviceFunctionality{functionality: onlySystemIdle},
			Timeout: 20 * time.Minute,
		}},
	})
}

func DeviceFunctionalityAfterSleep(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper

	testOpts := s.Param().(deviceFunctionality)

	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to init servo: ", err)
	}

	s.Log("Login to Chrome")
	cl, err := rpc.Dial(ctx, h.DUT, s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	client := power.NewUSBServiceClient(cl.Conn)
	if _, err := client.NewChrome(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer client.CloseChrome(ctx, &empty.Empty{})
	// Wait for the DUT to login completely.
	if err := testing.Sleep(ctx, 5*time.Second); err != nil {
		s.Error("Failed to wait for the DUT to login completely")
	}

	if testOpts.functionality != onlySystemIdle {
		usb2Re := regexp.MustCompile(`If 0.*Class=.*480M`)
		out, err := h.DUT.Conn().CommandContext(ctx, "lsusb", "-t").Output()
		if err != nil {
			s.Fatal("Failed to execute lsusb command: ", err)
		}
		if !usb2Re.MatchString(string(out)) {
			s.Fatal("Failed to find USB2 PenDrive using lsusb command")
		}
	}

	slpOpSetPre, pkgOpSetPre, err := powercontrol.SlpAndC10PackageValues(ctx, s.DUT())
	if err != nil {
		s.Fatal("Failed to get SLP counter and C10 package values before suspend-resume: ", err)
	}

	// Turning on DUT, in case test fails in middle.
	defer func() {
		waitCtx, cancel := context.WithTimeout(ctx, 1*time.Minute)
		defer cancel()
		if err := h.WaitConnect(waitCtx); err != nil {
			if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurPress); err != nil {
				s.Error("Failed to power button press: ", err)
			}
			if err := h.WaitConnect(waitCtx); err != nil {
				s.Error("Failed to wait connect DUT: ", err)
			}
		}
	}()

	if testOpts.functionality == lidCloseOpenWithUsb2 {
		if err := performLidCloseOpen(ctx, h); err != nil {
			s.Fatal("Failed to perform Lid Close Open: ", err)
		}
	} else if testOpts.functionality == systemIdleWithUsb2 {
		if err := performSystemIdle(ctx, h, false); err != nil {
			s.Fatal("Failed to perform System Idle: ", err)
		}
	} else {
		if err := performSystemIdle(ctx, h, true); err != nil {
			s.Fatal("Failed to perform Long duration System Idle: ", err)
		}
	}

	waitCtx, cancelWaitConnectShort := context.WithTimeout(ctx, 1*time.Minute)
	defer cancelWaitConnectShort()
	if err := h.WaitConnect(waitCtx); err != nil {
		if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurPress); err != nil {
			s.Fatal("Failed to power button press: ", err)
		}
		if err := h.WaitConnect(ctx); err != nil {
			s.Fatal("Failed to wait connect DUT: ", err)
		}
	}

	slpOpSetPost, pkgOpSetPost, err := powercontrol.SlpAndC10PackageValues(ctx, s.DUT())
	if err != nil {
		s.Fatal("Failed to get SLP counter and C10 package values after lid-open: ", err)
	}

	if slpOpSetPre == slpOpSetPost {
		s.Fatalf("Failed: SLP counter value %v should be different from the one before %v", slpOpSetPost, slpOpSetPre)
	}
	if slpOpSetPost == 0 {
		s.Fatal("Failed: SLP counter value must be non-zero, got: ", slpOpSetPost)
	}
	if pkgOpSetPre == pkgOpSetPost {
		s.Fatalf("Failed: Package C10 value %q must be different from the one before %q", pkgOpSetPost, pkgOpSetPre)
	}
	if pkgOpSetPost == "0x0" || pkgOpSetPost == "0" {
		s.Fatal("Failed: Package C10 should be non-zero, got: ", pkgOpSetPost)
	}

	if testOpts.functionality != onlySystemIdle {
		cl, err := rpc.Dial(ctx, h.DUT, s.RPCHint())
		if err != nil {
			s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
		}
		defer cl.Close(ctx)

		cr := power.NewUSBServiceClient(cl.Conn)
		if _, err := cr.ReuseChrome(ctx, &empty.Empty{}); err != nil {
			s.Fatal("Failed to reconnect to the Chrome session: ", err)
		}

		path, err := cr.USBMountPaths(ctx, &empty.Empty{})
		if err != nil {
			s.Fatal("Failed to get USB mount path: ", err)
		}

		testFileName := "test.dat"
		testfile, err := cr.GenerateTestFile(ctx, &power.TestFileRequest{FileName: testFileName})
		if err != nil {
			s.Fatal("Failed to generate test file: ", err)
		}

		if err := h.DUT.Conn().CommandContext(ctx, "cp", testfile.Path, path.MountPaths[0]).Run(); err != nil {
			s.Fatal("Failed to copy file to USB: ", err)
		}

		defer func(ctx context.Context) {
			if err := h.DUT.Conn().CommandContext(ctx, "rm", "-rf", testfile.Path, filepath.Join(path.MountPaths[0], testFileName)).Run(); err != nil {
				s.Error("Failed to delete testfile: ", err)
			}
		}(ctx)
	}
}

// performLidCloseOpen closes the DUT Lid and opens the Lid after confirming
// it went into suspend state.
func performLidCloseOpen(ctx context.Context, h *firmware.Helper) error {
	// Emulate DUT lid closing.
	if err := h.Servo.CloseLid(ctx); err != nil {
		return errors.Wrap(err, "failed to close DUT's lid")
	}
	testing.Poll(ctx, func(ctx context.Context) error {
		lidState, err := h.Servo.LidOpenState(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to check the lid state")
		}
		if lidState != string(servo.LidOpenNo) {
			return errors.Errorf("failed to check DUT lid state, expected: %q got: %q", servo.LidOpenNo, lidState)
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second})

	if err := powercontrol.WaitForSuspendState(ctx, h); err != nil {
		return errors.Wrap(err, "failed to wait for S0ix or S3 power state")
	}

	// Emulate DUT lid opening.
	if err := h.Servo.OpenLid(ctx); err != nil {
		return errors.Wrap(err, "failed to open DUT's lid")
	}
	testing.Poll(ctx, func(ctx context.Context) error {
		lidState, err := h.Servo.LidOpenState(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to check the lid state")
		}
		if lidState != string(servo.LidOpenYes) {
			return errors.Errorf("failed to check DUT lid state, expected: %q got: %q", servo.LidOpenYes, lidState)
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second})
	return nil
}

// performSystemIdle sets the power policy of the DUT to get into idle state
// and confirms it went into suspend state.
func performSystemIdle(ctx context.Context, h *firmware.Helper, isLongDuration bool) error {
	testing.ContextLog(ctx, "Setting power policy")
	if err := setPowerPolicy(ctx, h); err != nil {
		return errors.Wrap(err, "failed to setup power policy")
	}
	defer resetPowerPolicy(ctx, h)

	if isLongDuration {
		// After display goes OFF, keep DUT undisturbed for 10 minutes.
		testing.ContextLog(ctx, "Keeping DUT undisturbed for 10 minutes")
		if err := testing.Sleep(ctx, 10*time.Minute); err != nil {
			return errors.Wrap(err, "failed to sleep for 10 minutes")
		}
	}

	if err := powercontrol.WaitForSuspendState(ctx, h); err != nil {
		return errors.Wrap(err, "failed to wait for S0ix or S3 power state")
	}

	// waking DUT with power button.
	if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurPress); err != nil {
		return errors.Wrap(err, "failed to wake DUT with power button")
	}
	return nil
}

// setPowerPolicy sets power policy using set_power_policy command.
func setPowerPolicy(ctx context.Context, h *firmware.Helper) error {
	//disable_idle_suspend and restarting powerd.
	if err := h.DUT.Conn().CommandContext(ctx, "sh", "-c", fmt.Sprintf(
		"echo 0 > /var/lib/power_manager/disable_idle_suspend &&"+
			"restart powerd"),
	).Run(ssh.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to disable idle suspend and restart powerd")
	}

	// set_power_policy will fail right after restarting powerd.
	if err := testing.Sleep(ctx, 2*time.Second); err != nil {
		return errors.Wrap(err, "failed to sleep after restarting powerd")
	}

	idleDelay := 6
	if err := h.DUT.Conn().CommandContext(ctx, "set_power_policy",
		fmt.Sprintf("--battery_idle_delay=%d", idleDelay), fmt.Sprintf("--ac_idle_delay=%d", idleDelay),
	).Run(ssh.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to set power policy")
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if brightness, err := systemBrightness(ctx, h); err != nil {
			return errors.Wrap(err, "failed to get system current brightness in idle state")
		} else if brightness != 0 {
			return errors.Wrap(err, "failed to go to idle state")
		}
		return nil
	}, &testing.PollOptions{Interval: time.Second,
		Timeout: 5 * time.Minute} /*display goes off around 5 minutes*/); err != nil {
		return errors.Wrap(err, "failed to wait for DUT to go to idle state")
	}
	return nil
}

// resetPowerPolicy resets power policy using set_power_policy command.
func resetPowerPolicy(ctx context.Context, h *firmware.Helper) error {
	testing.ContextLog(ctx, "Resetting power policy")
	if err := h.DUT.Conn().CommandContext(ctx, "sh", "-c", fmt.Sprintf(
		"echo 1 > /var/lib/power_manager/disable_idle_suspend &&"+
			"restart powerd"),
	).Run(ssh.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to disable idle suspend and restart powerd")
	}

	// set_power_policy will fail right after restarting powerd.
	if err := testing.Sleep(ctx, 2*time.Second); err != nil {
		return errors.Wrap(err, "failed to sleep after restarting powerd")
	}

	if err := h.DUT.Conn().CommandContext(ctx, "set_power_policy", "reset").Run(ssh.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to set power policy")
	}
	return nil
}

// systemBrightness returns system display brightness value.
func systemBrightness(ctx context.Context, h *firmware.Helper) (int, error) {
	bnsOut, err := h.DUT.Conn().CommandContext(ctx, "backlight_tool", "--get_brightness").Output()
	if err != nil {
		return 0, errors.Wrap(err, "failed to execute backlight_tool command")
	}
	brightness, err := strconv.Atoi(strings.TrimSpace(string(bnsOut)))
	if err != nil {
		return 0, errors.Wrap(err, "failed to convert string to integer")
	}
	return brightness, nil
}
