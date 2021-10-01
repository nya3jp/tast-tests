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
		Func:         DUTWakeS3UsingPowerButton,
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

func DUTWakeS3UsingPowerButton(ctx context.Context, s *testing.State) {
	ctxCleanup := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Minute)
	defer cancel()
	dut := s.DUT()
	h := s.FixtValue().(*fixture.Value).Helper
	var (
		restartPowerd     = regexp.MustCompile("powerd start/running")
		wakeUpFromS3      = regexp.MustCompile("Waking up from system sleep state S3")
		requiredEventSets = [][]string{[]string{`Sleep`, `^Wake`},
			[]string{`ACPI Enter \| S3`, `ACPI Wake \| S3`},
		}
	)
	const (
		s3DmesgCmd            = "dmesg | grep S3"
		clrDemsgCmd           = "dmesg -C"
		configSuspendModeS3   = "echo 0 > /var/lib/power_manager/suspend_to_idle"
		restartPowerdCmd      = "restart powerd"
		configMemSleepS3      = "echo deep > /sys/power/mem_sleep"
		configSuspendModeS0ix = "echo 1 > /var/lib/power_manager/suspend_to_idle"
		configMemSleepS0ix    = "echo s2idle > /sys/power/mem_sleep"
		powerdSuspendModeCmd  = "check_powerd_config --suspend_to_idle; echo $?"
	)

	// Login to chrome OS.
	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)
	client := security.NewBootLockboxServiceClient(cl.Conn)
	if _, err := client.NewChromeLogin(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}

	defer func(ctx context.Context) {
		s.Log("Performing cleanup")
		if !dut.Connected(ctx) {
			if err := powerDutOn(ctx, h, dut); err != nil {
				s.Fatal("Failed to power on DUT at cleanup: ", err)
			}
		}
		// Perform switching back to S0ix state.
		if err := h.DUT.Conn().CommandContext(ctx, "bash", "-c", configSuspendModeS0ix).Run(); err != nil {
			s.Fatal("Failed to switch to S0ix: ", err)
		}
		restartOut, err := h.DUT.Conn().CommandContext(ctx, "bash", "-c", restartPowerdCmd).Output()
		if err != nil {
			s.Fatal("Failed to restart powerd at cleanup: ", err)
		}
		if !restartPowerd.MatchString(string(restartOut)) {
			s.Fatal("Failed to restart powerd at cleanup; expect 'powerd start/running', got: ", string(restartOut))
		}
		// Configure mem_sleep of S0ix state.
		if err := h.DUT.Conn().CommandContext(ctx, "bash", "-c", configMemSleepS0ix).Run(); err != nil {
			s.Fatal("Failed to config S0ix mem sleep state: ", err)
		}
	}(ctxCleanup)

	// Perform switching back to S3 state.
	if err := h.DUT.Conn().CommandContext(ctx, "bash", "-c", configSuspendModeS3).Run(); err != nil {
		s.Fatal("Failed to switch to S3: ", err)
	}
	restartOut, err := h.DUT.Conn().CommandContext(ctx, "bash", "-c", restartPowerdCmd).Output()
	if err != nil {
		s.Fatal("Failed to restart powerd: ", err)
	}
	if !restartPowerd.MatchString(string(restartOut)) {
		s.Fatal("Failed to restart powerd; expect 'powerd start/running', got: ", string(restartOut))
	}
	// Configure mem_sleep of S3 state.
	if err := h.DUT.Conn().CommandContext(ctx, "bash", "-c", configMemSleepS3).Run(); err != nil {
		s.Fatal("Failed to config S3 mem sleep state: ", err)
	}
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
		if err := h.DUT.Conn().CommandContext(ctx, "bash", "-c", clrDemsgCmd).Run(); err != nil {
			s.Fatal("Failed to clear dmesg: ", err)
		}
		// Check powerd configuration is switched to S3 for which expected configValue is 1.
		configValue, err := h.DUT.Conn().CommandContext(ctx, "bash", "-c", powerdSuspendModeCmd).Output()
		if err != nil {
			s.Fatal("Failed to check powerd config value: ", err)
		}
		actualValue := strings.TrimSpace(string(configValue))
		expectedValue := "1"
		if actualValue != expectedValue {
			s.Fatal("Failed: expect powerd configured to suspend to S3; S0ix found")
		}
		// Executing powerd_dbus_suspend command.
		suspendCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		defer cancel()
		if err := dut.Conn().CommandContext(suspendCtx, "powerd_dbus_suspend").Run(); err != nil && !errors.Is(err, context.DeadlineExceeded) {
			s.Fatal("Failed to suspend the DUT: ", err)
		}
		sdCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		if err := dut.WaitUnreachable(sdCtx); err != nil {
			s.Fatal("Failed to wait for the DUT being unreachable: ", err)
		}
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			pwrState, err := h.Servo.GetECSystemPowerState(ctx)
			if err != nil {
				return errors.Wrap(err, "failed to get power state via servo")
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
		dmesgOut, err := h.DUT.Conn().CommandContext(ctx, "bash", "-c", s3DmesgCmd).Output()
		if err != nil {
			s.Fatal("Failed to check S3 in dmesg log: ", err)
		}
		if !wakeUpFromS3.MatchString(string(dmesgOut)) {
			s.Fatalf("Failed to find %q pattern in dmesg log", wakeUpFromS3)
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
	for i := 0; i < 2; i++ {
		if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurPress); err != nil {
			return errors.Wrap(err, "failed to press power key via servo")
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
