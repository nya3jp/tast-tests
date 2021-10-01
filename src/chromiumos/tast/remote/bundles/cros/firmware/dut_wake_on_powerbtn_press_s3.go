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

func init() {
	testing.AddTest(&testing.Test{
		Func:         DUTWakeOnPowerbtnPressS3,
		Desc:         "Verifies waking DUT from S3 using power button press",
		Contacts:     []string{"pathan.jilani@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.security.BootLockboxService"},
		Vars:         []string{"servo"},
		Attr:         []string{"group:firmware", "firmware_experimental"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
		Fixture:      fixture.NormalMode,
		Timeout:      10 * time.Minute,
	})
}

func DUTWakeOnPowerbtnPressS3(ctx context.Context, s *testing.State) {
	ctxCleanup := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Minute)
	defer cancel()
	dut := s.DUT()
	h := s.FixtValue().(*fixture.Value).Helper
	var (
		WakeUpFromS3      = regexp.MustCompile("Waking up from system sleep state S3")
		requiredEventSets = [][]string{[]string{`Sleep`, `^Wake`},
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
	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)
	client := security.NewBootLockboxServiceClient(cl.Conn)
	if _, err := client.NewChromeLogin(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	if err := h.DUT.Conn().CommandContext(ctx, "bash", "-c", SwitchToS3Cmd).Run(); err != nil {
		s.Fatalf("Failed to execute %s command: %v", SwitchToS3Cmd, err)
	}
	restartOut, err := h.DUT.Conn().CommandContext(ctx, "bash", "-c", RestartPowerdCmd).Output()
	if err != nil {
		s.Fatalf("Failed to execute %s command: %v", RestartPowerdCmd, err)
	}
	if !RestartPowerd.MatchString(string(restartOut)) {
		s.Fatal("Failed to restart powerd; expect 'powerd start/running', got: ", string(restartOut))
	}
	if err := h.DUT.Conn().CommandContext(ctx, "bash", "-c", S3MemSleepCmd).Run(); err != nil {
		s.Fatalf("Failed to execute %s command: %v", S3MemSleepCmd, err)
	}
	defer func(ctx context.Context) {
		if !dut.Connected(ctx) {
			if err := powerDutOn(ctx, h, dut); err != nil {
				s.Fatal("Failed to power on DUT at cleanup: ", err)
			}
		}
		if err := h.DUT.Conn().CommandContext(ctx, "bash", "-c", SwitchToS0ixCmd).Run(); err != nil {
			s.Fatalf("Failed to execute %s command: %v", SwitchToS0ixCmd, err)
		}
		restartOut, err := h.DUT.Conn().CommandContext(ctx, "bash", "-c", RestartPowerdCmd).Output()
		if err != nil {
			s.Fatalf("Failed to execute %s command: %v", RestartPowerdCmd, err)
		}
		if !RestartPowerd.MatchString(string(restartOut)) {
			s.Fatal("Failed to restart powerd at cleanup; expect 'powerd start/running', got: ", string(restartOut))
		}
		if err := h.DUT.Conn().CommandContext(ctx, "bash", "-c", S0ixMemSleepCmd).Run(); err != nil {
			s.Fatalf("Failed to execute %s command: %v", S0ixMemSleepCmd, err)
		}
	}(ctxCleanup)
	const iter = 10
	for i := 1; i <= iter; i++ {
		s.Logf("Iteration: %d/%d", i, iter)
		r := h.Reporter
		var cutoffEvent reporters.Event
		oldEvents, err := r.EventlogList(ctx)
		if err != nil {
			s.Fatal("Failed finding last event: ", err)
		}
		if len(oldEvents) > 0 {
			cutoffEvent = oldEvents[len(oldEvents)-1]
		}
		if err := h.DUT.Conn().CommandContext(ctx, "bash", "-c", ClrDemsgCmd).Run(); err != nil {
			s.Fatalf("Failed to execute %s command: %v", ClrDemsgCmd, err)
		}
		configValue, err := h.DUT.Conn().CommandContext(ctx, "bash", "-c", PowerdConfigCmd).Output()
		if err != nil {
			s.Fatalf("Failed to execute %s command: %v", PowerdConfigCmd, err)
		}
		actualValue := strings.TrimSpace(string(configValue))
		expectedValue := "1"
		if actualValue != expectedValue {
			s.Fatalf("Failed to be in S3 state; expected PowerdConfig value %s; got %s", expectedValue, actualValue)
		}

		suspendCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		defer cancel()
		if err := dut.Conn().CommandContext(suspendCtx, "powerd_dbus_suspend").Run(); err != nil && !errors.Is(err, context.DeadlineExceeded) {
			s.Fatal("Failed to power off DUT: ", err)
		}
		sdCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		if err := dut.WaitUnreachable(sdCtx); err != nil {
			s.Fatal("Failed to wait for the DUT being unreachable: ", err)
		}
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			pwrState, err := h.Servo.GetECSystemPowerState(ctx)
			if err != nil {
				return errors.Wrap(err, "failed to get power state S3")
			}
			if pwrState != "S3" {
				return errors.New("System is not in S3")
			}
			return nil
		}, &testing.PollOptions{Timeout: 15 * time.Second}); err != nil {
			s.Fatal("Failed to enter S3 state: ", err)
		}
		if err := powerDutOn(ctx, h, dut); err != nil {
			s.Fatal("Failed to power on DUT after suspend: ", err)
		}
		dmesgOut, err := h.DUT.Conn().CommandContext(ctx, "bash", "-c", S3DmesgCmd).Output()
		if err != nil {
			s.Fatalf("Failed to execute %s command: %v", S3DmesgCmd, err)
		}
		if !WakeUpFromS3.MatchString(string(dmesgOut)) {
			s.Fatalf("Failed to find %q pattern in dmesg log", WakeUpFromS3)
		}
		events, err := r.EventlogListAfter(ctx, cutoffEvent)
		if err != nil {
			s.Fatal("Failed gathering events: ", err)
		}
		requiredEventsFound := false
		for _, requiredEventSet := range requiredEventSets {
			foundAllRequiredEventsInSet := true
			for _, requiredEvent := range requiredEventSet {
				eventRe := regexp.MustCompile(requiredEvent)
				if !eventMessageContainMatch(ctx, events, eventRe) {
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
			s.Fatal("Failed as required event missing")
		}
	}
}

func powerDutOn(ctx context.Context, h *firmware.Helper, dut *dut.DUT) error {
	waitCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurPress); err != nil {
		return errors.Wrap(err, "failed to press power key via servo")
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

func eventMessageContainMatch(ctx context.Context, events []reporters.Event, re *regexp.Regexp) bool {
	for _, event := range events {
		if re.MatchString(event.Message) {
			return true
		}
	}
	return false
}
