// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"strconv"
	"time"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/testing"
)

type cCDCapabilitiesGscFullConsoleParam struct {
	cap_state                        servo.CCDCapState
	commands_succeed_when_ccd_open   bool
	commands_succeed_when_ccd_locked bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func: CCDCapabilitiesGscFullConsole,
		Desc: "Test to verify GscFullConsole locks out restricted console commands.",
		Attr: []string{"group:firmware", "group:hwsec", "firmware_unstable"},
		Contacts: []string{
			"cros-hwsec@chromium.org",
			"mvertescher@google.com",
		},
		Fixture:      fixture.NormalMode,
		SoftwareDeps: []string{"gsc"},
		Timeout:      2 * time.Minute,
		Vars:         []string{"servo"},
		Params: []testing.Param{{
			Name: "cap_default",
			Val: cCDCapabilitiesGscFullConsoleParam{
				cap_state:                        servo.CapDefault,
				commands_succeed_when_ccd_open:   true,
				commands_succeed_when_ccd_locked: false,
			},
		}, {
			Name: "cap_always",
			Val: cCDCapabilitiesGscFullConsoleParam{
				cap_state:                        servo.CapAlways,
				commands_succeed_when_ccd_open:   true,
				commands_succeed_when_ccd_locked: true,
			},
		}, {
			Name: "cap_unless_locked",
			Val: cCDCapabilitiesGscFullConsoleParam{
				cap_state:                        servo.CapUnlessLocked,
				commands_succeed_when_ccd_open:   true,
				commands_succeed_when_ccd_locked: false,
			},
		}, {
			Name: "cap_if_opened",
			Val: cCDCapabilitiesGscFullConsoleParam{
				cap_state:                        servo.CapIfOpened,
				commands_succeed_when_ccd_open:   true,
				commands_succeed_when_ccd_locked: false,
			},
		}},
	})
}

func CCDCapabilitiesGscFullConsole(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper
	userParams := s.Param().(cCDCapabilitiesGscFullConsoleParam)

	if err := h.OpenCCD(ctx, true, false); err != nil {
		s.Fatal("Failed to get open CCD: ", err)
	}

	ccdSettings := map[servo.CCDCap]servo.CCDCapState{"GscFullConsole": userParams.cap_state}
	if err := h.Servo.SetCCDCapability(ctx, ccdSettings); err != nil {
		s.Fatal("Failed to set `GscFullConsole` capability state: ", err)
	}

	run_gsc_full_console_commands(ctx, s, userParams.commands_succeed_when_ccd_open)

	if err := h.Servo.LockCCD(ctx); err != nil {
		s.Fatal("Failed to lock ccd: ", err)
	}

	// Open CCD when finished
	defer func() {
		if err := h.OpenCCD(ctx, true, false); err != nil {
			s.Fatal("Failed to get open CCD: ", err)
		}
	}()

	run_gsc_full_console_commands(ctx, s, userParams.commands_succeed_when_ccd_locked)
}

func run_gsc_full_console_commands(ctx context.Context, s *testing.State, expect_success bool) {
	h := s.FixtValue().(*fixture.Value).Helper

	for _, tc := range []struct {
		command       string
		success_regex string
		failure_regex string
	}{
		{
			command:       "idle s",
			success_regex: "idle action: sleep",
			failure_regex: "Console is locked|Access Denied",
		},
		{
			command:       "recbtnforce enable",
			success_regex: "RecBtn",
			failure_regex: "Access Denied",
		},
		{
			command:       "rddkeepalive true",
			success_regex: "Forcing",
			failure_regex: "Parameter 1 invalid|Access Denied",
		},
	} {
		regex := tc.success_regex
		if !expect_success {
			regex = tc.failure_regex
		}

		if err := h.Servo.CheckGSCCommandOutput(ctx, tc.command, []string{regex}); err != nil {
			s.Fatal("Failed to match GSC command output, expected command `"+tc.command+"` to succeed = "+strconv.FormatBool(expect_success)+": ", err)
		}
	}
}
