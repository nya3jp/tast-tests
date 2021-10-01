// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/remote/firmware/reporters"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/ui"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type s3StabilityTestParams struct {
	tabletMode bool
	val        int
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         S3SuspendResume,
		Desc:         "Verifies DUT S3 entry and exit with suspend-resume",
		Contacts:     []string{"pathan.jilani@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.ui.ScreenLockService"},
		Vars:         []string{"servo"},
		Attr:         []string{"group:firmware", "firmware_experimental"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
		Params: []testing.Param{{
			Name:    "stability_test_clamshell_mode",
			Fixture: fixture.NormalMode,
			Val: s3StabilityTestParams{
				tabletMode: false,
				val:        10,
			},
			Timeout: 10 * time.Minute,
		}, {
			Name:    "stability_test_tablet_mode",
			Fixture: fixture.NormalMode,
			Val: s3StabilityTestParams{
				tabletMode: true,
				val:        10,
			},
			Timeout: 10 * time.Minute,
		}, {
			Name:    "entry_exit_clamshell_mode",
			Fixture: fixture.NormalMode,
			Val: s3StabilityTestParams{
				tabletMode: false,
				val:        1,
			},
		}, {
			Name:    "entry_exit_tablet_mode",
			Fixture: fixture.NormalMode,
			Val: s3StabilityTestParams{
				tabletMode: true,
				val:        1,
			},
		}, {
			Name:    "stress_test",
			Fixture: fixture.NormalMode,
			Val: s3StabilityTestParams{
				tabletMode: false,
				val:        100,
			},
			Timeout: 28 * time.Minute,
		}},
	})
}

func S3SuspendResume(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper
	dut := s.DUT()
	testOpt := s.Param().(s3StabilityTestParams)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Minute)
	defer cancel()

	var (
		WakeUpFromS3      = regexp.MustCompile("Waking up from system sleep state S3")
		requiredEventSets = [][]string{[]string{`Sleep`, `^Wake`},
			[]string{`ACPI Enter \| S3`, `ACPI Wake \| S3`},
		}
		PrematureWake  = regexp.MustCompile("Premature wakes: 0")
		SuspndFailure  = regexp.MustCompile("Suspend failures: 0")
		FrmwreLogError = regexp.MustCompile("Firmware log errors: 0")
	)

	const (
		suspendToIdle   = "0"
		ClrDemsgCmd     = "dmesg -C"
		S3DmesgCmd      = "dmesg | grep S3"
		PowerdConfigCmd = "check_powerd_config --suspend_to_idle; echo $?"
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
	defer cl.Close(cleanupCtx)

	screenLockService := ui.NewScreenLockServiceClient(cl.Conn)
	if _, err := screenLockService.NewChrome(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to login chrome: ", err)
	}
	defer screenLockService.CloseChrome(ctx, &empty.Empty{})

	r := h.Reporter
	var cutoffEvent reporters.Event
	oldEvents, err := r.EventlogList(ctx)
	if err != nil {
		s.Fatal("Failed finding last event: ", err)
	}

	if len(oldEvents) > 0 {
		cutoffEvent = oldEvents[len(oldEvents)-1]
	}

	if err := h.DUT.Conn().CommandContext(ctx, "sh", "-c", fmt.Sprintf(
		"mkdir -p /tmp/power_manager && "+
			"echo %q > /tmp/power_manager/suspend_to_idle && "+
			"mount --bind /tmp/power_manager /var/lib/power_manager && "+
			"restart powerd", suspendToIdle),
	).Run(ssh.DumpLogOnError); err != nil {
		s.Fatal("Failed to set suspend to idle: ", err)
	}

	defer func(ctx context.Context) {
		s.Log("Performing cleanup")
		if !dut.Connected(ctx) {
			waitCtx, cancel := context.WithTimeout(ctx, 1*time.Minute)
			defer cancel()
			if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurPress); err != nil {
				s.Fatal("Failed to power normal press: ", err)
			}
			if err := dut.WaitConnect(waitCtx); err != nil {
				s.Fatal("Failed to wait connect DUT: ", err)
			}
		}

		if err := h.DUT.Conn().CommandContext(ctx, "sh", "-c",
			"umount /var/lib/power_manager && restart powerd",
		).Run(ssh.DumpLogOnError); err != nil {
			s.Log("Failed to restore powerd settings: ", err)
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
		s.Fatalf("Failed to be in S3 state. PowerdConfig want %q; got %q", expectedValue, actualValue)
	}

	// expected time sleep 8 seconds to ensure DUT switch to S3.
	// otherwise premature wake, suspend failure errors are expected.
	if err := testing.Sleep(ctx, 8*time.Second); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	val := fmt.Sprintf("%d", testOpt.val)
	testing.ContextLog(ctx, "Executing suspend_stress_test")
	stressOut, err := h.DUT.Conn().CommandContext(ctx, "suspend_stress_test", "-c", val).Output()
	if err != nil {
		s.Fatal("Failed to execute suspend_stress_test command: ", err)
	}

	var errorCodes []*regexp.Regexp
	errorCodes = []*regexp.Regexp{PrematureWake, SuspndFailure, FrmwreLogError}
	for _, errMsg := range errorCodes {
		if !(errMsg).MatchString(string(stressOut)) {
			s.Fatalf("Failed for failures; expected %q but got non-zero %s", errMsg, string(stressOut))
		}
	}

	dmesgOut, err := h.DUT.Conn().CommandContext(ctx, "bash", "-c", S3DmesgCmd).Output()
	if err != nil {
		s.Fatalf("Failed to execute %q command: %v", S3DmesgCmd, err)
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
			reRequiredEvent := regexp.MustCompile(requiredEvent)
			if !eventMessagesContainMatch(ctx, events, reRequiredEvent) {
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

// eventMessagesContainMatch verifies whether mosys event log contains matching eventlog.
func eventMessagesContainMatch(ctx context.Context, events []reporters.Event, re *regexp.Regexp) bool {
	for _, event := range events {
		if re.MatchString(event.Message) {
			return true
		}
	}
	return false
}
