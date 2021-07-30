// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"regexp"
	"time"

	fwCommon "chromiumos/tast/common/firmware"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/pre"
	"chromiumos/tast/remote/firmware/reporters"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// eventLogParams contains all the data needed to run a single test iteration.
type eventLogParams struct {
	resetType     firmware.ResetType
	bootToMode    fwCommon.BootMode
	suspendResume bool
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
		Data: []string{firmware.ConfigFile},
		HardwareDeps: hwdep.D(
			// Eventlog is broken/wontfix on veyron devices.
			// See http://b/35585376#comment14 for more info.
			hwdep.SkipOnPlatform("veyron_fievel"),
			hwdep.SkipOnPlatform("veyron_tiger"),
		),
		ServiceDeps:  []string{"tast.cros.firmware.UtilsService", "tast.cros.firmware.BiosService"},
		SoftwareDeps: []string{"crossystem", "flashrom"},
		Vars:         []string{"servo"},
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
			},
			// Test eventlog upon rec->normal reboot
			{
				Name:      "rec_normal",
				Pre:       pre.RecMode(),
				ExtraAttr: []string{"firmware_usb"},
				Val: eventLogParams{
					bootToMode:        fwCommon.BootModeRecovery,
					requiredEventSets: [][]string{[]string{`System boot`, `Chrome OS Recovery Mode \| Recovery Button`}},
					prohibitedEvents:  `Developer Mode|Sleep|FW Wake|ACPI Wake \| S3`,
				},
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
			// TODO(b/174800291): Test eventlog upon suspend/resume w/ disable_suspend_to_idle
			// TODO(b/174800291): Test eventlog with hardware watchdog
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
		if err = ms.RebootToMode(ctx, param.bootToMode); err != nil {
			s.Fatalf("Error during transition to %s: %+v", param.bootToMode, err)
		}
	} else if param.suspendResume {
		if err = h.DUT.Conn().CommandContext(ctx, "powerd_dbus_suspend", "-wakeup_timeout=10").Run(ssh.DumpLogOnError); err != nil {
			s.Fatal("Failed to suspend: ", err)
		}
		if err = testing.Sleep(ctx, 5*time.Second); err != nil {
			s.Fatal("Failed to sleep: ", err)
		}
	}
	events, err := r.EventlogListSince(ctx, cutoffTime)
	if err != nil {
		s.Fatal("Gathering events after normal reboot: ", err)
	}
	requiredEventsFound := false
	for _, requiredEventSet := range param.requiredEventSets {
		for _, requiredEvent := range requiredEventSet {
			reRequiredEvent := regexp.MustCompile(requiredEvent)
			if eventMessagesContainReMatch(ctx, events, reRequiredEvent /*exceptionRe=*/, nil) {
				requiredEventsFound = true
			}
		}
	}
	if !requiredEventsFound {
		s.Errorf("Required event missing: %+v", events)
	}
	reProhibitedEvents := regexp.MustCompile(param.prohibitedEvents)
	var allowedRe *regexp.Regexp
	if param.allowedEvents != "" {
		allowedRe = regexp.MustCompile(param.allowedEvents)
	}
	if eventMessagesContainReMatch(ctx, events, reProhibitedEvents, allowedRe) {
		s.Errorf("Incorrect event logged: %+v", events)
	}
}
