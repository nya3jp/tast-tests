// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package intel

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/remote/firmware/reporters"
	"chromiumos/tast/remote/powercontrol"
	"chromiumos/tast/ssh"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DarkResumeFunctionalityWithTimeout,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies dark resume feature functionality with wakeup-timeout parameter",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome", "reboot"},
		ServiceDeps:  []string{"tast.cros.security.BootLockboxService"},
		Vars:         []string{"servo"},
		HardwareDeps: hwdep.D(hwdep.X86(), hwdep.ChromeEC()),
		Fixture:      fixture.NormalMode,
		Timeout:      10 * time.Minute,
	})
}

func DarkResumeFunctionalityWithTimeout(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Minute)
	defer cancel()

	dut := s.DUT()

	firmwareHelper := s.FixtValue().(*fixture.Value).Helper
	if err := firmwareHelper.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}

	// Performing chrome login.
	if err := powercontrol.ChromeOSLogin(ctx, dut, s.RPCHint()); err != nil {
		s.Fatal("Failed to login to chrome: ", err)
	}

	eventReporter := firmwareHelper.Reporter
	var cutoffEvent reporters.Event
	oldEvents, err := eventReporter.EventlogList(ctx)
	if err != nil {
		s.Fatal("Failed finding last event: ", err)
	}

	if len(oldEvents) > 0 {
		cutoffEvent = oldEvents[len(oldEvents)-1]
	}

	cmdRun := func(cmd string) {
		if err := dut.Conn().CommandContext(ctx, "bash", "-c", cmd).Run(); err != nil {
			s.Fatalf("Failed to execute %s command: %v", cmd, err)
		}
	}

	const (
		enableDarkResumeCommand  = "echo 0 > /var/lib/power_manager/disable_dark_resume"
		disableDarkResumeCommand = "echo 1 > /var/lib/power_manager/disable_dark_resume"
		restartPowerdCommand     = "restart powerd"
	)

	defer func(ctx context.Context) {
		if !dut.Connected(ctx) {
			if err := powercontrol.PowerOntoDUT(ctx, firmwareHelper.ServoProxy, dut); err != nil {
				s.Fatal("Failed to power-on DUT at cleanup: ", err)
			}
		}
		if err := dut.Conn().CommandContext(ctx, "sh", "-c", "umount /var/lib/power_manager && restart powerd").Run(ssh.DumpLogOnError); err != nil {
			s.Log("Failed to restore powerd settings: ", err)
		}
		cmdRun(disableDarkResumeCommand)
		cmdRun(restartPowerdCommand)
	}(cleanupCtx)

	if err := dut.Conn().CommandContext(ctx, "sh", "-c", fmt.Sprintf(
		"mkdir -p /tmp/power_manager && "+
			"echo 1 > /tmp/power_manager/suspend_to_idle && "+
			"mount --bind /tmp/power_manager /var/lib/power_manager && "+
			"restart powerd"),
	).Run(ssh.DumpLogOnError); err != nil {
		s.Fatal("Failed to set suspend to idle: ", err)
	}

	const expectedConfigValue = 0
	if err := powercontrol.VerifyPowerdConfigSuspendValue(ctx, dut, expectedConfigValue); err != nil {
		s.Fatal("Failed to verfiy power config value for S0ix: ", err)
	}

	cmdRun(enableDarkResumeCommand)
	cmdRun(restartPowerdCommand)

	darkResumeEnabledString := "Dark resume enabled"
	if err := verifyDarkResumeStatus(ctx, dut, darkResumeEnabledString); err != nil {
		s.Fatal("Failed to check dark resume enabled status before suspend: ", err)
	}

	isPowerPress := true
	// Perform dark resume suspend with servo ENTER key as wake source.
	if err := performDarkResumeSuspend(ctx, firmwareHelper, dut, !isPowerPress); err != nil {
		s.Fatal("Failed to perform dark resume suspend with ENTER key as wake source: ", err)
	}

	// Perform dark resume suspend with servo power button as wake source.
	if err := performDarkResumeSuspend(ctx, firmwareHelper, dut, isPowerPress); err != nil {
		s.Fatal("Failed to perform dark resume suspend with power button as wake source: ", err)
	}

	events, err := eventReporter.EventlogListAfter(ctx, cutoffEvent)
	if err != nil {
		s.Fatal("Failed gathering events: ", err)
	}

	// requiredECEvents is expected list of EC events after suspend-resume.
	var requiredECEvents = []string{"S0ix Enter",
		"S0ix Exit",
		`Wake Source \| Power Button \| 0`,
		`Wake Source \| GPE \# \| 112`,
	}

	foundRequiredEvents := true
	for _, requiredEvent := range requiredECEvents {
		reRequiredEvent := regexp.MustCompile(requiredEvent)
		if !eventMessageContainsMatch(ctx, events, reRequiredEvent) {
			foundRequiredEvents = false
			s.Errorf("Unexpected events: want %q, got %q", requiredECEvents, events)
			break
		}
	}

	if !foundRequiredEvents {
		s.Error("Failed as required event missing")
	}
}

// verifyDarkResumeStatus verifies dark resume status with provided matchString.
func verifyDarkResumeStatus(ctx context.Context, dut *dut.DUT, matchString string) error {
	const darkResumeStatusFile = "/var/log/power_manager/powerd.LATEST"
	return testing.Poll(ctx, func(ctx context.Context) error {
		out, err := linuxssh.ReadFile(ctx, dut.Conn(), darkResumeStatusFile)
		if err != nil {
			return errors.Wrap(err, "failed to execute dark resume status check command")
		}
		if !strings.Contains(string(out), matchString) {
			return errors.Errorf("failed to check for %q match in %q output", matchString, string(out))
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second})
}

// eventMessageContainsMatch verifies whether mosys event log contains matching eventlog.
func eventMessageContainsMatch(ctx context.Context, events []reporters.Event, re *regexp.Regexp) bool {
	for _, event := range events {
		if re.MatchString(event.Message) {
			return true
		}
	}
	return false
}

// performDarkResumeSuspend perform powerd_dbus_suspend with disable_dark_resume and wakeup_timeout paramater.
// Checks and compare SLP and C10 package values before and after suspend.
// Wakes DUT with ENTER key/power button press via servo.
func performDarkResumeSuspend(ctx context.Context, firmwareHelper *firmware.Helper, dut *dut.DUT, isPowerPress bool) error {
	slpOpSetPre, pkgOpSetPre, err := powercontrol.SlpAndC10PackageValues(ctx, dut)
	if err != nil {
		return errors.Wrap(err, "failed to get SLP counter and C10 package values before suspend-resume")
	}

	testing.ContextLog(ctx, "Suspend DUT with dark resume command")
	cmd := dut.Conn().CommandContext(ctx, "powerd_dbus_suspend", "--disable_dark_resume=false", "--wakeup_timeout=10")
	if err := cmd.Start(); err != nil {
		return errors.Wrap(err, "failed to execute dark resume command")
	}
	cmd.Wait()

	sdCtx, cancel := context.WithTimeout(ctx, 40*time.Second)
	defer cancel()
	if err := dut.WaitUnreachable(sdCtx); err != nil {
		return errors.Wrap(err, "failed to wait DUT to become unreachable")
	}

	if err := powercontrol.WaitForSuspendState(ctx, firmwareHelper); err != nil {
		return errors.Wrap(err, "failed to verify EC power state after suspend")
	}

	if !isPowerPress {
		// Press on ENTER key via servo and wake DUT.
		testing.ContextLog(ctx, "Waking DUT with ENTER key press via servo")
		if err := wakeDUTWithEnterKeyPress(ctx, firmwareHelper, dut); err != nil {
			return errors.Wrap(err, "failed to press ENTER to wake DUT")
		}
	} else {
		// Press on power button via servo and wake DUT.
		testing.ContextLog(ctx, "Waking DUT with power button press via servo")
		if err := powercontrol.PowerOntoDUT(ctx, firmwareHelper.ServoProxy, dut); err != nil {
			return errors.Wrap(err, "failed to press power button to wake DUT")
		}
	}

	wtCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	if err := dut.WaitConnect(wtCtx); err != nil {
		return errors.Wrap(err, "failed to wait DUT to connect")
	}

	slpOpSetPost, pkgOpSetPost, err := powercontrol.SlpAndC10PackageValues(ctx, dut)
	if err != nil {
		return errors.Wrap(err, "failed to get SLP counter and C10 package values after suspend-resume")
	}

	if slpOpSetPre == slpOpSetPost {
		return errors.Errorf("failed: SLP counter value %q should be different from the one before suspend %q", slpOpSetPost, slpOpSetPre)
	}

	if slpOpSetPost == 0 {
		return errors.Errorf("failed SLP counter value must be non-zero, got: %q", slpOpSetPost)
	}

	if pkgOpSetPre == pkgOpSetPost {
		return errors.Errorf("failed: Package C10 value %q must be different from the one before suspend %q", pkgOpSetPost, pkgOpSetPre)
	}

	if pkgOpSetPost == "0x0" || pkgOpSetPost == "0" {
		return errors.New("Failed: Package C10 should be non-zero")
	}

	inDarkResumeString := "In dark resume"
	if err := verifyDarkResumeStatus(ctx, dut, inDarkResumeString); err != nil {
		return errors.Wrap(err, "failed to check dark resume enabled status after suspend")
	}
	return nil
}

// wakeDUTWithEnterKeyPress performs power normal press to wake DUT.
func wakeDUTWithEnterKeyPress(ctx context.Context, firmwareHelper *firmware.Helper, dut *dut.DUT) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		if err := firmwareHelper.Servo.KeypressWithDuration(ctx, servo.Enter, servo.DurPress); err != nil {
			return errors.Wrap(err, "failed to press ENTER to wake DUT")
		}
		waitCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		if err := dut.WaitConnect(waitCtx); err != nil {
			return errors.Wrap(err, "failed to wait connect DUT")
		}
		return nil
	}, &testing.PollOptions{Timeout: 2 * time.Minute})
}
