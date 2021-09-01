// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"regexp"
	"time"

	fwCommon "chromiumos/tast/common/firmware"
	"chromiumos/tast/common/servo"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/pre"
	"chromiumos/tast/remote/firmware/reporters"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// eventLogParams contains all the data needed to run a single test iteration.
type eventLogParams struct {
	resetType            firmware.ResetType
	bootToMode           fwCommon.BootMode
	suspendResume        bool
	disableSuspendToIdle bool
	hardwareWatchdog     bool
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
			"gredelston@google.com", // Test author
			"cros-fw-engprod@google.com",
		},
		Attr: []string{"group:firmware", "firmware_experimental"},
		HardwareDeps: hwdep.D(
			// Eventlog is broken/wontfix on veyron devices.
			// See http://b/35585376#comment14 for more info.
			hwdep.SkipOnPlatform("veyron_fievel"),
			hwdep.SkipOnPlatform("veyron_tiger"),
		),
		Data:         pre.Data,
		ServiceDeps:  pre.ServiceDeps,
		SoftwareDeps: pre.SoftwareDeps,
		Vars:         pre.Vars,
		Params: []testing.Param{
			// Test eventlog upon normal->normal reboot
			{
				Name:              "normal",
				ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel("leona")),
				Pre:               pre.NormalMode(),
				Val: eventLogParams{
					resetType:         firmware.WarmReset,
					requiredEventSets: [][]string{[]string{`System boot`}},
					prohibitedEvents:  `Developer Mode|Recovery Mode|Sleep| Wake`,
				},
			},
			{
				// Allow some normally disallowed events on lenoa. b/184778308
				Name:              "leona_normal",
				ExtraHardwareDeps: hwdep.D(hwdep.Model("leona")),
				Pre:               pre.NormalMode(),
				Val: eventLogParams{
					resetType:         firmware.WarmReset,
					requiredEventSets: [][]string{[]string{`System boot`}},
					prohibitedEvents:  `Developer Mode|Recovery Mode|Sleep| Wake`,
					allowedEvents:     `^ACPI Wake \| Deep S5$`,
				},
			},
			// Test eventlog upon dev->dev reboot
			{
				Name:              "dev",
				ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel("leona")),
				Pre:               pre.DevModeGBB(),
				Val: eventLogParams{
					resetType:         firmware.WarmReset,
					requiredEventSets: [][]string{[]string{`System boot`, `Chrome OS Developer Mode`}},
					prohibitedEvents:  `Recovery Mode|Sleep| Wake`,
				},
			},
			// Allow some normally disallowed events on lenoa. b/184778308
			{
				Name:              "leona_dev",
				ExtraHardwareDeps: hwdep.D(hwdep.Model("leona")),
				Pre:               pre.DevModeGBB(),
				Val: eventLogParams{
					resetType:         firmware.WarmReset,
					requiredEventSets: [][]string{[]string{`System boot`, `Chrome OS Developer Mode`}},
					prohibitedEvents:  `Recovery Mode|Sleep| Wake`,
					allowedEvents:     `^ACPI Wake \| Deep S5$`,
				},
			},
			// Test eventlog upon normal->rec reboot
			{
				Name:      "normal_rec",
				Pre:       pre.NormalMode(),
				ExtraAttr: []string{"firmware_usb"},
				Val: eventLogParams{
					bootToMode:        fwCommon.BootModeRecovery,
					requiredEventSets: [][]string{[]string{`System boot`, `Chrome OS Recovery Mode \| Recovery Button`}},
					prohibitedEvents:  `Developer Mode|Sleep|FW Wake|ACPI Wake \| S3`,
				},
				Timeout: 60 * time.Minute,
			},
			// Test eventlog upon rec->normal reboot
			{
				Name:              "rec_normal",
				ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel("leona")),
				Pre:               pre.RecMode(),
				ExtraAttr:         []string{"firmware_usb"},
				Val: eventLogParams{
					bootToMode:        fwCommon.BootModeNormal,
					requiredEventSets: [][]string{[]string{`System boot`}},
					prohibitedEvents:  `Developer Mode|Recovery Mode|Sleep| Wake`,
				},
				Timeout: 6 * time.Minute,
			},
			{
				// Allow some normally disallowed events on lenoa. b/184778308
				Name:              "leona_rec_normal",
				ExtraHardwareDeps: hwdep.D(hwdep.Model("leona")),
				Pre:               pre.RecMode(),
				ExtraAttr:         []string{"firmware_usb"},
				Val: eventLogParams{
					bootToMode:        fwCommon.BootModeNormal,
					requiredEventSets: [][]string{[]string{`System boot`}},
					prohibitedEvents:  `Developer Mode|Recovery Mode|Sleep| Wake`,
					allowedEvents:     `^ACPI Wake \| Deep S5$`,
				},
				Timeout: 6 * time.Minute,
			},
			// Test eventlog upon suspend/resume
			{
				Name: "suspend_resume",
				Pre:  pre.NormalMode(),
				Val: eventLogParams{
					suspendResume: true,
					requiredEventSets: [][]string{
						[]string{`^Wake`, `Sleep`},
						[]string{`ACPI Enter \| S3`, `ACPI Wake \| S3`},
						[]string{`S0ix Enter`, `S0ix Exit`},
					},
					prohibitedEvents: `System |Developer Mode|Recovery Mode`,
				},
			},
			// Test eventlog upon suspend/resume w/ disable_suspend_to_idle
			{
				Name: "suspend_resume_noidle",
				Pre:  pre.NormalMode(),
				Val: eventLogParams{
					suspendResume:        true,
					disableSuspendToIdle: true,
					requiredEventSets: [][]string{
						[]string{`ACPI Enter \| S3`, `ACPI Wake \| S3`},
					},
					prohibitedEvents: `System |Developer Mode|Recovery Mode`,
				},
			},
			// Test eventlog with hardware watchdog
			{
				Name:              "watchdog",
				Pre:               pre.NormalMode(),
				ExtraSoftwareDeps: []string{"watchdog"},
				Val: eventLogParams{
					hardwareWatchdog: true,
					requiredEventSets: [][]string{
						[]string{`System boot|Hardware watchdog reset`},
					},
				},
			},
		},
	})
}

// eventMessagesContainReMatch returns true if any event's message matches the regexp, and doesn't match the `exceptionRe`.
func eventMessagesContainReMatch(ctx context.Context, events []reporters.Event, re, exceptionRe *regexp.Regexp) bool {
	for _, event := range events {
		if re.MatchString(event.Message) && (exceptionRe == nil || !exceptionRe.MatchString(event.Message)) {
			return true
		}
	}
	return false
}

func Eventlog(ctx context.Context, s *testing.State) {
	// Create mode-switcher
	v := s.PreValue().(*pre.Value)
	h := v.Helper
	ms, err := firmware.NewModeSwitcher(ctx, h)
	if err != nil {
		s.Fatal("Creating mode switcher: ", err)
	}
	r := h.Reporter
	param := s.Param().(eventLogParams)

	cutoffTime, err := r.Now(ctx)
	if err != nil {
		s.Fatal("Reporting time at start of test: ", err)
	}
	if param.resetType != "" {
		if err := ms.ModeAwareReboot(ctx, param.resetType); err != nil {
			s.Fatal("Error resetting DUT: ", err)
		}
	} else if param.bootToMode != "" {
		// If booting into recovery, check the USB Key
		if param.bootToMode == fwCommon.BootModeRecovery {
			if err := h.SetupUSBKey(ctx, s.CloudStorage()); err != nil {
				s.Fatal("USBKey not working: ", err)
			}
		}
		if err = ms.RebootToMode(ctx, param.bootToMode); err != nil {
			s.Fatalf("Error during transition to %s: %+v", param.bootToMode, err)
		}
	} else if param.suspendResume {
		if err := h.Servo.WatchdogRemove(ctx, servo.WatchdogCCD); err != nil {
			s.Error("Failed to remove watchdog for ccd: ", err)
		}
		if param.disableSuspendToIdle {
			if err = h.DUT.Conn().CommandContext(ctx, "sh", "-c",
				"mkdir -p /tmp/power_manager && "+
					"echo 0 > /tmp/power_manager/suspend_to_idle && "+
					"mount --bind /tmp/power_manager /var/lib/power_manager && "+
					"restart powerd",
			).Run(ssh.DumpLogOnError); err != nil {
				s.Fatal("Failed to disable suspend to idle: ", err)
			}
			defer func(ctx context.Context) {
				if err := h.DUT.Conn().CommandContext(ctx, "sh", "-c",
					"umount /var/lib/power_manager && restart powerd",
				).Run(ssh.DumpLogOnError); err != nil {
					s.Fatal("Failed to restore suspend to idle: ", err)
				}
			}(ctx)
		}
		s.Log("Suspending DUT")
		if err = h.DUT.Conn().CommandContext(ctx, "powerd_dbus_suspend", "-wakeup_timeout=10").Run(ssh.DumpLogOnError); err != nil && !errors.Is(err, context.DeadlineExceeded) {
			s.Fatal("Failed to suspend: ", err)
		}
		h.CloseRPCConnection(ctx)

		s.Log("Reconnecting to DUT")
		if err := h.WaitConnect(ctx); err != nil {
			s.Fatal("Failed to reconnect to DUT: ", err)
		}
		s.Log("Reconnected to DUT")
	} else if param.hardwareWatchdog {
		if err := h.Servo.WatchdogRemove(ctx, servo.WatchdogCCD); err != nil {
			s.Error("Failed to remove watchdog for ccd: ", err)
		}
		// Daisydog is the watchdog service
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
		if err := h.WaitConnect(ctx); err != nil {
			s.Fatal("Failed to reconnect to DUT: ", err)
		}
		s.Log("Reconnected to DUT")
	}
	events, err := r.EventlogListSince(ctx, cutoffTime)
	if err != nil {
		s.Fatal("Gathering events: ", err)
	}

	// Complicated rules here.
	// One of the param.requiredEventSets must be found.
	// Within that event set, all the regexs need to match to be considered found.
	requiredEventsFound := false
	for _, requiredEventSet := range param.requiredEventSets {
		foundAllRequiredEventsInSet := true
		for _, requiredEvent := range requiredEventSet {
			reRequiredEvent := regexp.MustCompile(requiredEvent)
			if !eventMessagesContainReMatch(ctx, events, reRequiredEvent /*exceptionRe=*/, nil) {
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
		s.Errorf("Required event missing: %+v", events)
	}
	if param.prohibitedEvents != "" {
		reProhibitedEvents := regexp.MustCompile(param.prohibitedEvents)
		var allowedRe *regexp.Regexp
		if param.allowedEvents != "" {
			allowedRe = regexp.MustCompile(param.allowedEvents)
		}
		if eventMessagesContainReMatch(ctx, events, reProhibitedEvents, allowedRe) {
			s.Errorf("Incorrect event logged: %+v", events)
		}
	}
}
