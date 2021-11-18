// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"strconv"
	"time"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/remote/firmware/reporters"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WriteProtect,
		Desc:         "Compare ec flash size to expected ec size from a chip-to-size map",
		Contacts:     []string{"tij@google.com", "cros-fw-engprod@google.com"},
		Attr:         []string{"group:firmware", "firmware_unstable"},
		SoftwareDeps: []string{"crossystem"},
		ServiceDeps:  []string{"tast.cros.firmware.UtilsService"},
		Params: []testing.Param{
			{
				Name:    "normal_mode",
				Fixture: fixture.NormalMode,
			},
			{
				Name:    "dev_gbb",
				Fixture: fixture.DevModeGBB,
			},
		},
	})
}

const (
	softwareSync  time.Duration = 6 * time.Second
	rebootTimeout time.Duration = 2 * time.Second
)

func WriteProtect(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper
	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	if err := h.RequireRPCUtils(ctx); err != nil {
		s.Fatal("Requiring RPC utils: ", err)
	}

	s.Log("Enable Write Protect")
	if err := setWriteProtect(ctx, h, servo.FWWPStateOn); err != nil {
		s.Fatal("Failed to set FW write protect state: ", err)
	}
	s.Log("Expect write protect state to be enabled")
	if err := checkCrossystem(ctx, h, 1); err != nil {
		s.Fatal("Failed to check crossystem: ", err)
	}
	s.Log("Disable Write Protect")
	if err := setWriteProtect(ctx, h, servo.FWWPStateOff); err != nil {
		s.Fatal("Failed to set FW write protect state: ", err)
	}
	s.Log("Expect write protect state to be disabled")
	if err := checkCrossystem(ctx, h, 0); err != nil {
		s.Fatal("Failed to check crossystem: ", err)
	}
	s.Log("Enable Write Protect")
	if err := setWriteProtect(ctx, h, servo.FWWPStateOn); err != nil {
		s.Fatal("Failed to set FW write protect state: ", err)
	}
	s.Log("Expect write protect state to be enabled")
	if err := checkCrossystem(ctx, h, 1); err != nil {
		s.Fatal("Failed to check crossystem: ", err)
	}
}

func checkCrossystem(ctx context.Context, h *firmware.Helper, expectedWpsw int) error {
	testing.ContextLog(ctx, "Create new Reporter to check crossystem")
	r := reporters.New(h.DUT)
	testing.ContextLog(ctx, "Check crossystem for write protect state param")
	paramMap, err := r.Crossystem(ctx, reporters.CrossystemParamWpswCur)
	if err != nil {
		return errors.Wrapf(err, "failed to get crossystem %v value", reporters.CrossystemParamWpswCur)
	}
	currWpsw, err := strconv.Atoi(paramMap[reporters.CrossystemParamWpswCur])
	if err != nil {
		return errors.Wrap(err, "failed to convert crossystem wpsw value to integer value")
	}
	testing.ContextLogf(ctx, "Current write protect state: %v, Expected state: %v", currWpsw, expectedWpsw)
	if currWpsw != expectedWpsw {
		return errors.Errorf("expected WP state to %v, is actually %v", expectedWpsw, currWpsw)
	}
	return nil
}

func setWriteProtect(ctx context.Context, h *firmware.Helper, fwpState servo.FWWPStateValue) error {
	testing.ContextLog(ctx, "Create new mode switcher")
	ms, err := firmware.NewModeSwitcher(ctx, h)
	if err != nil {
		return errors.Wrap(err, "failed to create mode switcher")
	}
	testing.ContextLogf(ctx, "Setting fwp state to %q", fwpState)
	if err := h.Servo.SetFWWPState(ctx, fwpState); err != nil {
		return errors.Wrap(err, "failed to enable firmware write protect")
	}
	testing.ContextLog(ctx, "Reboot")
	if err := ms.ModeAwareReboot(ctx, firmware.WarmReset); err != nil {
		return errors.Wrap(err, "failed to Reset")
	}
	return nil
}
