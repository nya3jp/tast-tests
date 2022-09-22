// Copyright 2022 The ChromiumOS Authors
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

type cCDCapabilitiesOverrideBatt struct {
	capState                     servo.CCDCapState
	commandsSucceedWhenCcdOpen   bool
	commandsSucceedWhenCcdLocked bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func: CCDCapabilitiesOverrideBatt,
		Desc: "Test to verify OverrideBatt CCD capability locks out restricted console commands",
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
			Val: cCDCapabilitiesOverrideBatt{
				capState:                     servo.CapDefault,
				commandsSucceedWhenCcdOpen:   true,
				commandsSucceedWhenCcdLocked: false,
			},
		}, {
			Name: "cap_always",
			Val: cCDCapabilitiesOverrideBatt{
				capState:                     servo.CapAlways,
				commandsSucceedWhenCcdOpen:   true,
				commandsSucceedWhenCcdLocked: true,
			},
		}, {
			Name: "cap_unless_locked",
			Val: cCDCapabilitiesOverrideBatt{
				capState:                     servo.CapUnlessLocked,
				commandsSucceedWhenCcdOpen:   true,
				commandsSucceedWhenCcdLocked: false,
			},
		}, {
			Name: "cap_if_opened",
			Val: cCDCapabilitiesOverrideBatt{
				capState:                     servo.CapIfOpened,
				commandsSucceedWhenCcdOpen:   true,
				commandsSucceedWhenCcdLocked: false,
			},
		}},
	})
}

func CCDCapabilitiesOverrideBatt(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper
	userParams := s.Param().(cCDCapabilitiesOverrideBatt)

	if err := h.OpenCCD(ctx, true, false); err != nil {
		s.Fatal("Failed to open CCD: ", err)
	}

	ccdSettings := map[servo.CCDCap]servo.CCDCapState{"OverrideBatt": userParams.capState}
	if err := h.Servo.SetCCDCapability(ctx, ccdSettings); err != nil {
		s.Fatal("Failed to set `OverrideBatt` capability state: ", err)
	}

	// Check that we can run the `bpforce` command successfully
	runGscBpforceConnect(ctx, s, userParams.commandsSucceedWhenCcdOpen)

	if err := h.Servo.LockCCD(ctx); err != nil {
		s.Fatal("Failed to lock CCD: ", err)
	}

	// Open CCD when finished
	defer func() {
		if err := h.OpenCCD(ctx, true, false); err != nil {
			s.Fatal("Failed to open CCD: ", err)
		}
	}()

	runGscBpforceConnect(ctx, s, userParams.commandsSucceedWhenCcdLocked)
}

func runGscBpforceConnect(ctx context.Context, s *testing.State, expectSuccess bool) {
	h := s.FixtValue().(*fixture.Value).Helper
	command := "bpforce connect"
	failureRegex := "Access Denied"
	regex := "batt pres:"
	if !expectSuccess {
		regex = failureRegex
	}

	if err := h.Servo.CheckGSCCommandOutput(ctx, command, []string{regex}); err != nil {
		s.Fatal("Failed to match GSC command output, expected command `"+command+"` to succeed = "+strconv.FormatBool(expectSuccess)+": ", err)
	}
}
