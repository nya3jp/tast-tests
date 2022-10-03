// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"time"

	gossh "golang.org/x/crypto/ssh"

	fwCommon "chromiumos/tast/common/firmware"
	"chromiumos/tast/common/servo"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/remote/firmware/reporters"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// eventLogParams contains all the data needed to run a single test iteration.
type eventLogParams struct {
	resetType        firmware.ResetType
	bootToMode       fwCommon.BootMode
	suspendResume    bool
	suspendToIdle    string
	hardwareWatchdog bool
	// All of the regexes in one of the sets must be present. Ex.
	// [][]string{[]string{`Case 1A`, `Case 1B`}, []string{`Case 2A`, `Case 2[BC]`}}
	// Any of these events would pass:
	// Case 1A, Case 1B
	// Case 2A, Case 2B
	// Case 2A, Case 2C
	requiredEventSets [][]string
	prohibitedEvents  string
	allowedEvents     string
}

func init() {
	testing.AddTest(&testing.Test{
		Func: Eventlog,
		Desc: "Ensure that eventlog is written on boot and suspend/resume",
		Contacts: []string{
			"gredelston@google.com", // Test author.
			"cros-fw-engprod@google.com",
		},
		Attr: []string{"group:firmware"},
		HardwareDeps: hwdep.D(
			// Eventlog is broken/wontfix on veyron devices.
			// See http://b/35585376#comment14 for more info.
			hwdep.SkipOnPlatform("veyron_fievel"),
			hwdep.SkipOnPlatform("veyron_tiger"),
		),
		SoftwareDeps: []string{"crossystem", "flashrom"},
		Vars:         []string{"firmware.skipFlashUSB"},
		Params: []testing.Param{
			// Test eventlog upon normal->normal reboot.
			{
				Name:      "normal",
				ExtraAttr: []string{"firmware_ec"},
				// Disable on leona (b/184778308) and coral (b/250684696)
				ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel("leona", "astronaut", "babymega", "babytiger", "blacktiplte", "nasher", "robo360")),
				Fixture:           fixture.NormalMode,
				Val: eventLogParams{
					resetType:         firmware.WarmReset,
					requiredEventSets: [][]string{{`System boot`}},
					prohibitedEvents:  `Developer Mode|Recovery Mode|Sleep| Wake`,
				},
			},
			{
				// Allow some normally disallowed events on leona. b/184778308
				Name:              "leona_normal",
				ExtraAttr:         []string{"firmware_ec"},
				ExtraHardwareDeps: hwdep.D(hwdep.Model("leona")),
				Fixture:           fixture.NormalMode,
				Val: eventLogParams{
					resetType:         firmware.WarmReset,
					requiredEventSets: [][]string{{`System boot`}},
					prohibitedEvents:  `Developer Mode|Recovery Mode|Sleep| Wake`,
					allowedEvents:     `^ACPI Wake \| Deep S5$`,
				},
			},
			// Test eventlog upon dev->dev reboot.
			{
				Name:              "dev",
				ExtraAttr:         []string{"firmware_ec"},
				ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel("leona")),
				Fixture:           fixture.DevModeGBB,
				Val: eventLogParams{
					resetType:         firmware.WarmReset,
					requiredEventSets: [][]string{{`System boot`, `Chrome ?OS Developer Mode|boot_mode=Developer`}},
					prohibitedEvents:  `Recovery Mode|Sleep| Wake`,
				},
			},
			// Allow some normally disallowed events on leona. b/184778308
			{
				Name:              "leona_dev",
				ExtraAttr:         []string{"firmware_ec"},
				ExtraHardwareDeps: hwdep.D(hwdep.Model("leona")),
				Fixture:           fixture.DevModeGBB,
				Val: eventLogParams{
					resetType:         firmware.WarmReset,
					requiredEventSets: [][]string{{`System boot`, `Chrome ?OS Developer Mode|boot_mode=Developer`}},
					prohibitedEvents:  `Recovery Mode|Sleep| Wake`,
					allowedEvents:     `^ACPI Wake \| Deep S5$`,
				},
			},
			// Test eventlog upon normal->rec reboot.
			{
				Name:      "normal_rec",
				ExtraAttr: []string{"firmware_unstable", "firmware_usb"},
				Fixture:   fixture.NormalMode,
				Val: eventLogParams{
					bootToMode:        fwCommon.BootModeRecovery,
					requiredEventSets: [][]string{{`System boot`, `(?i)Chrome ?OS Recovery Mode \| Recovery Button|boot_mode=Manual recovery`}},
					prohibitedEvents:  `Developer Mode|Sleep|FW Wake|ACPI Wake \| S3`,
				},
				Timeout: 60 * time.Minute,
			},
			// Test eventlog upon rec->normal reboot.
			{
				Name:      "rec_normal",
				ExtraAttr: []string{"firmware_unstable", "firmware_usb"},
				Fixture:   fixture.RecModeNoServices,
				Val: eventLogParams{
					bootToMode:        fwCommon.BootModeNormal,
					requiredEventSets: [][]string{{`System boot`}},
					prohibitedEvents:  `Developer Mode|Recovery Mode|Sleep`,
				},
				Timeout: 6 * time.Minute,
			},
			// Test eventlog upon suspend/resume w/ default value of suspend_to_idle.
			// treeya: ACPI Enter | S3, EC Event | Power Button, ACPI Wake | S3, Wake Source | Power Button | 0
			// kindred: S0ix Enter, S0ix Exit, Wake Source | Power Button | 0, EC Event | Power Button
			// leona: S0ix Enter, S0ix Exit, Wake Source | Power Button | 0, EC Event | Power Button
			// eldrid: S0ix Enter, S0ix Exit, Wake Source | Power Button | 0, EC Event | Power Button
			// hayato: Sleep, Wake
			{
				Name:      "suspend_resume",
				ExtraAttr: []string{"firmware_unstable"},
				Fixture:   fixture.NormalMode,
				Val: eventLogParams{
					suspendResume: true,
					requiredEventSets: [][]string{
						{`Sleep`, `^Wake`},
						{`ACPI Enter \| S3`, `ACPI Wake \| S3`},
						{`S0ix Enter`, `S0ix Exit`},
					},
					prohibitedEvents: `System |Developer Mode|Recovery Mode`,
				},
			},
			// Test eventlog upon suspend/resume w/ suspend_to_idle.
			// On supported machines, this should go to S0ix or stay in S0.
			// x86 duts: S0ix Enter, S0ix Exit, Wake Source | Power Button | 0, EC Event | Power Button
			// hayato: FAIL Sleep, System boot
			// treeya: FAIL Nothing logged
			{
				Name:      "suspend_resume_idle",
				ExtraAttr: []string{"firmware_unstable"},
				Fixture:   fixture.NormalMode,
				Val: eventLogParams{
					suspendResume: true,
					suspendToIdle: "1",
					requiredEventSets: [][]string{
						{`Sleep`, `^Wake`},
						{`S0ix Enter`, `S0ix Exit`},
					},
					prohibitedEvents: `System |Developer Mode|Recovery Mode`,
				},
			},
			// Test eventlog upon suspend/resume w/o suspend_to_idle.
			// This should power down all the way to S3.
			// eldrid: FAIL ACPI Enter | S3, EC Event | Power Button, ACPI Wake | S3, Wake Source | Power Button | 0 -> Gets stuck and doesn't boot.
			// hayato: Sleep, Wake
			// x86 duts: ACPI Enter | S3, EC Event | Power Button, ACPI Wake | S3, Wake Source | Power Button | 0
			{
				Name:      "suspend_resume_noidle",
				ExtraAttr: []string{"firmware_unstable"},
				Fixture:   fixture.NormalMode,
				Val: eventLogParams{
					suspendResume: true,
					suspendToIdle: "0",
					requiredEventSets: [][]string{
						{`Sleep`, `^Wake`},
						{`ACPI Enter \| S3`, `ACPI Wake \| S3`},
					},
					prohibitedEvents: `System |Developer Mode|Recovery Mode`,
				},
			},
			// Test eventlog with hardware watchdog.
			{
				Name:              "watchdog",
				ExtraAttr:         []string{"firmware_ec"},
				Fixture:           fixture.NormalMode,
				ExtraSoftwareDeps: []string{"watchdog"},
				Val: eventLogParams{
					hardwareWatchdog: true,
					requiredEventSets: [][]string{
						{`System boot|Hardware watchdog reset`},
					},
				},
			},
		},
	})
}

// eventMessagesContainReMatch returns true if any event's message matches the regexp.
func eventMessagesContainReMatch(ctx context.Context, events []reporters.Event, re *regexp.Regexp) bool {
	for _, event := range events {
		if re.MatchString(event.Message) {
			return true
		}
	}
	return false
}

func Eventlog(ctx context.Context, s *testing.State) {
	// Create mode-switcher.
	v := s.FixtValue().(*fixture.Value)
	h := v.Helper
	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servod")
	}
	ms, err := firmware.NewModeSwitcher(ctx, h)
	if err != nil {
		s.Fatal("Creating mode switcher: ", err)
	}
	r := h.Reporter
	param := s.Param().(eventLogParams)

	var cutoffEvent reporters.Event
	oldEvents, err := r.EventlogList(ctx)
	if err != nil {
		s.Fatal("Finding last event: ", err)
	}
	if len(oldEvents) > 0 {
		cutoffEvent = oldEvents[len(oldEvents)-1]
		s.Log("Found previous event: ", cutoffEvent)
	}
	if param.resetType != "" {
		if err := ms.ModeAwareReboot(ctx, param.resetType); err != nil {
			s.Fatal("Error resetting DUT: ", err)
		}
	} else if param.bootToMode != "" {
		// If booting into recovery, check the USB Key.
		if param.bootToMode == fwCommon.BootModeRecovery {
			skipFlashUSB := false
			if skipFlashUSBStr, ok := s.Var("firmware.skipFlashUSB"); ok {
				skipFlashUSB, err = strconv.ParseBool(skipFlashUSBStr)
				if err != nil {
					s.Fatalf("Invalid value for var firmware.skipFlashUSB: got %q, want true/false", skipFlashUSBStr)
				}
			}
			cs := s.CloudStorage()
			if skipFlashUSB {
				cs = nil
			}
			if err := h.SetupUSBKey(ctx, cs); err != nil {
				s.Fatal("USBKey not working: ", err)
			}
		}
		if err := ms.RebootToMode(ctx, param.bootToMode); err != nil {
			s.Fatalf("Error during transition to %s: %+v", param.bootToMode, err)
		}
	} else if param.suspendResume {
		if err := h.Servo.WatchdogRemove(ctx, servo.WatchdogCCD); err != nil {
			s.Error("Failed to remove watchdog for ccd: ", err)
		}
		if param.suspendToIdle != "" {
			if err := h.DUT.Conn().CommandContext(ctx, "sh", "-c", fmt.Sprintf(
				"mkdir -p /tmp/power_manager && "+
					"echo %q > /tmp/power_manager/suspend_to_idle && "+
					"mount --bind /tmp/power_manager /var/lib/power_manager && "+
					"restart powerd", param.suspendToIdle),
			).Run(ssh.DumpLogOnError); err != nil {
				s.Fatal("Failed to set suspend to idle: ", err)
			}
			defer func(ctx context.Context) {
				if err := h.DUT.Conn().CommandContext(ctx, "sh", "-c",
					"umount /var/lib/power_manager && restart powerd",
				).Run(ssh.DumpLogOnError); err != nil {
					s.Log("Failed to restore powerd settings: ", err)
				}
			}(ctx)
			// Suspend will fail right after restarting powerd.
			testing.Sleep(ctx, 2*time.Second)
		}
		h.CloseRPCConnection(ctx)

		s.Log("Suspending DUT")
		shortCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		if err := h.DUT.Conn().CommandContext(shortCtx, "powerd_dbus_suspend").Run(ssh.DumpLogOnError); err != nil &&
			!errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, &gossh.ExitMissingError{}) {
			s.Fatal("Failed to suspend: ", err)
		}

		// Let the DUT stay in suspend a little while. 10s seems to be enough to allow wake up. Shorter times might work also.
		if err := testing.Sleep(ctx, 10*time.Second); err != nil {
			s.Fatal("Failed to sleep: ", err)
		}

		powerState, err := h.Servo.GetECSystemPowerState(ctx)
		if err != nil {
			s.Error("Failed to get power state: ", err)
		}
		s.Log("Power state: ", powerState)

		s.Log("Pressing ENTER key to wake DUT")
		if err := h.Servo.KeypressWithDuration(ctx, servo.Enter, servo.DurTab); err != nil {
			s.Fatal("Failed to press enter key")
		}

		s.Log("Reconnecting to DUT")
		shortCtx, cancel = context.WithTimeout(ctx, 60*time.Second)
		defer cancel()
		if err := h.WaitConnect(shortCtx); err != nil {
			s.Fatal("Failed to reconnect to DUT: ", err)
		}
		s.Log("Reconnected to DUT")
	} else if param.hardwareWatchdog {
		if err := h.Servo.WatchdogRemove(ctx, servo.WatchdogCCD); err != nil {
			s.Error("Failed to remove watchdog for ccd: ", err)
		}
		// Daisydog is the watchdog service.
		cmd := `nohup sh -c 'sleep 2
			sync
			stop daisydog
			sleep 60 > /dev/watchdog' >/dev/null 2>&1 </dev/null &`
		if err := h.DUT.Conn().CommandContext(ctx, "bash", "-c", cmd).Run(); err != nil {
			s.Fatal("Failed to panic DUT: ", err)
		}
		s.Log("Waiting for DUT to become unreachable")
		h.CloseRPCConnection(ctx)

		if err := h.DUT.WaitUnreachable(ctx); err != nil {
			s.Fatal("Failed to wait for DUT to become unreachable: ", err)
		}
		s.Log("DUT became unreachable (as expected)")

		s.Log("Reconnecting to DUT")
		shortCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()
		if err := h.WaitConnect(shortCtx); err != nil {
			s.Fatal("Failed to reconnect to DUT: ", err)
		}
		s.Log("Reconnected to DUT")
	}
	// Sometimes events are missing if you check too quickly after boot.
	var events []reporters.Event
	if err := testing.Poll(ctx, func(context.Context) error {
		var err error
		events, err = r.EventlogListAfter(ctx, cutoffEvent)
		if err != nil {
			return testing.PollBreak(err)
		}
		if len(events) == 0 {
			return errors.New("no new events found")
		}
		return nil
	}, &testing.PollOptions{
		Timeout: 1 * time.Minute, Interval: 5 * time.Second,
	}); err != nil {
		s.Fatal("Gathering events: ", err)
	}
	for _, event := range events {
		s.Log("Found event: ", event)
	}

	// Complicated rules here.
	// One of the param.requiredEventSets must be found.
	// Within that event set, all the regexs need to match to be considered found.
	requiredEventsFound := false
	for _, requiredEventSet := range param.requiredEventSets {
		foundAllRequiredEventsInSet := true
		for _, requiredEvent := range requiredEventSet {
			reRequiredEvent := regexp.MustCompile(requiredEvent)
			if !eventMessagesContainReMatch(ctx, events, reRequiredEvent) {
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
		s.Error("Required event missing")
	}
	if param.prohibitedEvents != "" {
		reProhibitedEvents := regexp.MustCompile(param.prohibitedEvents)
		var allowedRe *regexp.Regexp
		if param.allowedEvents != "" {
			allowedRe = regexp.MustCompile(param.allowedEvents)
		}
		for _, event := range events {
			if reProhibitedEvents.MatchString(event.Message) && (allowedRe == nil || !allowedRe.MatchString(event.Message)) {
				s.Errorf("Incorrect event logged: %+v", event)
			}
		}
	}
}
