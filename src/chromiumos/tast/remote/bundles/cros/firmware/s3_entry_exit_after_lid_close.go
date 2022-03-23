// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/remote/firmware/reporters"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/security"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type lidCloseTestParams struct {
	tabletMode bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         S3EntryExitAfterLidClose,
		Desc:         "Verifies DUT S3 entry exit after Lid close and open",
		Contacts:     []string{"pathan.jilani@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.security.BootLockboxService", "tast.cros.firmware.BiosService", "tast.cros.firmware.UtilsService"},
		Attr:         []string{"group:firmware", "firmware_unstable"},
		Vars:         []string{"servo"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
		Params: []testing.Param{{
			Name:    "clamshell_mode",
			Fixture: fixture.NormalMode,
			Val: lidCloseTestParams{
				tabletMode: false,
			},
			Timeout: 10 * time.Minute,
		}, {
			Name:    "tablet_mode",
			Fixture: fixture.NormalMode,
			Val: lidCloseTestParams{
				tabletMode: true,
			},
			Timeout: 10 * time.Minute,
		}},
	})
}

func S3EntryExitAfterLidClose(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper
	dut := s.DUT()
	testOpt := s.Param().(lidCloseTestParams)

	var (
		WakeUpFromS3      = regexp.MustCompile("Waking up from system sleep state S3")
		requiredEventSets = [][]string{
			[]string{`Sleep`, `^Wake`},
			[]string{`ACPI Enter \| S3`, `ACPI Wake \| S3`},
		}
		RestartPowerd = regexp.MustCompile("powerd start/running")
	)

	const (
		S3DmesgCmd       = "dmesg | grep S3"
		ClrDemsgCmd      = "dmesg -C"
		SwitchToS3Cmd    = "echo 0 > /var/lib/power_manager/suspend_to_idle"
		RestartPowerdCmd = "restart powerd"
		S3MemSleepCmd    = "echo deep > /sys/power/mem_sleep"
		SwitchToS0ixCmd  = "echo 1 > /var/lib/power_manager/suspend_to_idle"
		S0ixMemSleepCmd  = "echo s2idle > /sys/power/mem_sleep"
		PowerdConfigCmd  = "check_powerd_config --suspend_to_idle; echo $?"
	)

	// Get the initial tablet_mode_angle settings to restore at the end of test.
	re := regexp.MustCompile(`tablet_mode_angle=(\d+) hys=(\d+)`)
	out, err := dut.Conn().CommandContext(ctx, "ectool", "motionsense", "tablet_mode_angle").Output()
	if err != nil {
		s.Fatal("Failed to retrieve tablet_mode_angle settings: ", err)
	}
	m := re.FindSubmatch(out)
	if len(m) != 3 {
		s.Fatalf("Failed to get initial tablet_mode_angle settings: got submatches %+v", m)
	}
	initLidAngle := m[1]
	initHys := m[2]

	// Set tabletModeAngle to 0 to force the DUT into tablet mode.
	if testOpt.tabletMode {
		testing.ContextLog(ctx, "Put DUT into tablet mode")
		if err := dut.Conn().CommandContext(ctx, "ectool", "motionsense", "tablet_mode_angle", "0", "0").Run(); err != nil {
			s.Fatal("Failed to set DUT into tablet mode: ", err)
		}
	}

	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}

	defer cl.Close(ctx)
	client := security.NewBootLockboxServiceClient(cl.Conn)
	if _, err := client.NewChromeLogin(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}

	eventReporter := h.Reporter
	var cutoffEvent reporters.Event
	oldEvents, err := eventReporter.EventlogList(ctx)
	if err != nil {
		s.Fatal("Failed finding last event: ", err)
	}

	if len(oldEvents) > 0 {
		cutoffEvent = oldEvents[len(oldEvents)-1]
	}

	if err := h.DUT.Conn().CommandContext(ctx, "bash", "-c", SwitchToS3Cmd).Run(); err != nil {
		s.Fatalf("Failed to execute %q command: %v", SwitchToS3Cmd, err)
	}

	restartOut, err := h.DUT.Conn().CommandContext(ctx, "bash", "-c", RestartPowerdCmd).Output()
	if err != nil {
		s.Fatalf("Failed to execute %q command: %v", RestartPowerdCmd, err)
	}

	if !RestartPowerd.MatchString(string(restartOut)) {
		s.Fatal("Failed powerd to restart: ", err)
	}

	if err := h.DUT.Conn().CommandContext(ctx, "bash", "-c", S3MemSleepCmd).Run(); err != nil {
		s.Fatalf("Failed to execute %q command: %v", S3MemSleepCmd, err)
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Minute)
	defer cancel()
	defer func(ctx context.Context) {
		s.Log("Performing cleanup")
		if err := h.Servo.SetString(ctx, "lid_open", "yes"); err != nil {
			s.Fatal("Failed to open lid at cleanup: ", err)
		}

		if !dut.Connected(ctx) {
			s.Log("Powering up DUT at cleanup")
			if err := powerOnDut(ctx, h, dut); err != nil {
				s.Fatal("Failed to power on DUT at cleanup: ", err)
			}
		}

		if err := h.DUT.Conn().CommandContext(ctx, "bash", "-c", SwitchToS0ixCmd).Run(); err != nil {
			s.Fatalf("Failed to execute %q command: %v", SwitchToS0ixCmd, err)
		}

		restartOut, err := h.DUT.Conn().CommandContext(ctx, "bash", "-c", RestartPowerdCmd).Output()
		if err != nil {
			s.Fatalf("Failed to execute %q command: %v", RestartPowerdCmd, err)
		}

		if !RestartPowerd.MatchString(string(restartOut)) {
			s.Fatal("Failed to restart powerd: ", err)
		}

		if err := h.DUT.Conn().CommandContext(ctx, "bash", "-c", S0ixMemSleepCmd).Run(); err != nil {
			s.Fatalf("Failed to execute %q command: %v", S0ixMemSleepCmd, err)
		}

		if err := dut.Conn().CommandContext(ctx, "ectool", "motionsense", "tablet_mode_angle", string(initLidAngle), string(initHys)).Run(); err != nil {
			s.Fatal("Failed to restore tablet_mode_angle to the original settings: ", err)
		}
	}(cleanupCtx)

	if err := h.DUT.Conn().CommandContext(ctx, "bash", "-c", ClrDemsgCmd).Run(); err != nil {
		s.Fatalf("Failed to execute %q command: %v", ClrDemsgCmd, err)
	}

	configValue, err := h.DUT.Conn().CommandContext(ctx, "bash", "-c", PowerdConfigCmd).Output()
	if err != nil {
		s.Fatalf("Failed to execute %q command: %v", PowerdConfigCmd, err)
	}

	actualValue := strings.TrimSpace(string(configValue))
	expectedValue := "1"
	if actualValue != expectedValue {
		s.Fatalf("Failed to enter S3 state. PowerdConfig want %q; got %q", expectedValue, actualValue)
	}

	// expected time sleep 5 seconds to ensure dut switch to s3.
	if err := testing.Sleep(ctx, 5*time.Second); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	s.Log("Closing lid, waiting for DUT to become unreachable")
	if err := h.Servo.SetString(ctx, "lid_open", "no"); err != nil {
		s.Fatal("Failed to close lid: ", err)
	}

	sdCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	s.Log("Waiting for DUT unreachable")
	if err := dut.WaitUnreachable(sdCtx); err != nil {
		s.Fatal("Failed wait for unreachable: ", err)
	}

	s.Log("Waiting for DUT to be in S3 state after suspend")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		pwrState, err := h.Servo.GetECSystemPowerState(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get power state S3 error")
		}
		if pwrState != "S3" {
			return errors.New("System is not in S3")
		}
		return nil
	}, &testing.PollOptions{Timeout: 15 * time.Second}); err != nil {
		s.Fatal("Failed to enter S3 state: ", err)
	}

	s.Log("Opening lid")
	if err := h.Servo.SetString(ctx, "lid_open", "yes"); err != nil {
		s.Fatal("Failed to open lid: ", err)
	}

	dmesgOut, err := h.DUT.Conn().CommandContext(ctx, "bash", "-c", S3DmesgCmd).Output()
	if err != nil {
		s.Fatalf("Failed to execute %q command: %v", S3DmesgCmd, err)
	}

	if !WakeUpFromS3.MatchString(string(dmesgOut)) {
		s.Errorf("Failed to find %q pattern in dmesg log", WakeUpFromS3)
	}

	events, err := eventReporter.EventlogListAfter(ctx, cutoffEvent)
	if err != nil {
		s.Fatal("Failed gathering events: ", err)
	}

	requiredEventsFound := false
	for _, requiredEventSet := range requiredEventSets {
		foundAllRequiredEventsInSet := true
		for _, requiredEvent := range requiredEventSet {
			reRequiredEvent := regexp.MustCompile(requiredEvent)
			if !eventMessageContainsMatch(ctx, events, reRequiredEvent) {
				foundAllRequiredEventsInSet = false
				break
			}
		}
		if foundAllRequiredEventsInSet {
			requiredEventsFound = true
			break
		}
	}

	if !requiredEventsFound {
		s.Error("Failed as required event missing")
	}
}

// powerOnDut performs power normal press to wake DUT.
func powerOnDut(ctx context.Context, h *firmware.Helper, dut *dut.DUT) error {
	waitCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurPress); err != nil {
		return errors.Wrap(err, "failed to power normal press")
	}
	if err := dut.WaitConnect(waitCtx); err != nil {
		testing.ContextLog(ctx, "Unable to wake up DUT. Retrying")
		if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurPress); err != nil {
			return errors.Wrap(err, "failed to power normal press")
		}
		if err := dut.WaitConnect(waitCtx); err != nil {
			return errors.Wrap(err, "failed to wait connect DUT")
		}
	}
	return nil
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
