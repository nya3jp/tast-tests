// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"regexp"

	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/pre"
	"chromiumos/tast/remote/firmware/reporters"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// eventLogParams contains all the data needed to run a single test iteration.
type eventLogParams struct {
	additionalNormalEvents string
}

func init() {
	testing.AddTest(&testing.Test{
		Func: Eventlog,
		Desc: "Ensure that eventlog is written on boot and suspend/resume",
		Contacts: []string{
			"gredelston@google.com", // Test author
			"cros-fw-engprod@google.com",
		},
		Attr: []string{"group:firmware", "firmware_experimental", "firmware_usb"},
		HardwareDeps: hwdep.D(
			// Eventlog is broken/wontfix on veyron devices.
			// See http://b/35585376#comment14 for more info.
			hwdep.SkipOnPlatform("veyron_fievel"),
			hwdep.SkipOnPlatform("veyron_tiger"),
		),
		Pre:          pre.NormalMode(),
		Data:         pre.Data,
		ServiceDeps:  pre.ServiceDeps,
		SoftwareDeps: pre.SoftwareDeps,
		Vars:         pre.Vars,
		Params: []testing.Param{
			{
				ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel("leona")),
				Val:               eventLogParams{},
			},
			{
				// Allow some normally disallowed events on lenoa. b/184778308
				Name:              "leona",
				ExtraHardwareDeps: hwdep.D(hwdep.Model("leona")),
				Val: eventLogParams{
					additionalNormalEvents: `^ACPI Wake \| Deep S5$`,
				},
			},
		},
	})
}

var (
	// Expected and unexpected messages upon normal-mode reboot.
	reSystemBootMessage    = regexp.MustCompile("System boot")
	reNotSystemBootMessage = regexp.MustCompile("Developer Mode|Recovery Mode|Sleep| Wake")
)

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

	// Check eventlog behavior on normal boot
	cutoffTime, err := r.Now(ctx)
	if err != nil {
		s.Fatal("Reporting time at start of test: ", err)
	}
	if err := ms.ModeAwareReboot(ctx, firmware.WarmReset); err != nil {
		s.Fatal("Error resetting DUT: ", err)
	}
	events, err := r.EventlogListSince(ctx, cutoffTime)
	if err != nil {
		s.Fatal("Gathering events after normal reboot: ", err)
	}
	if !eventMessagesContainReMatch(ctx, events, reSystemBootMessage /*exceptionRe=*/, nil) {
		s.Error("No 'System boot' event on normal boot")
	}
	var exceptionRe *regexp.Regexp
	if param.additionalNormalEvents != "" {
		exceptionRe = regexp.MustCompile(param.additionalNormalEvents)
	}
	if eventMessagesContainReMatch(ctx, events, reNotSystemBootMessage, exceptionRe) {
		s.Errorf("Incorrect event logged on normal boot: %+v", events)
	}

	// TODO(b/174800291): Test eventlog upon dev->dev reboot
	// TODO(b/174800291): Test eventlog upon normal->rec reboot
	// TODO(b/174800291): Test eventlog upon rec->normal reboot
	// TODO(b/174800291): Test eventlog upon suspend/resume
	// TODO(b/174800291): Test eventlog with hardware watchdog
}
