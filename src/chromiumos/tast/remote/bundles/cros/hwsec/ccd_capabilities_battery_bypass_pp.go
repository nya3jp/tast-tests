// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/testing"
)

type cCDCapabilitiesBatteryBypassPPParam struct {
	isBatteryBypassPPEnabled               bool
	isBatteryConnected                     bool
	shouldCCDOpenRequirePhysicalPresence   bool
	shouldCCDUnlockRequirePhysicalPresence bool
	shouldGsctoolRequirePhysicalPresence   bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func: CCDCapabilitiesBatteryBypassPP,
		Desc: "Test to verify BatteryBypassPP CCD capability locks out restricted console commands.",
		Attr: []string{"group:firmware", "group:hwsec", "firmware_ccd", "firmware_cr50"},
		Contacts: []string{
			"cros-hwsec@chromium.org",
			"mvertescher@google.com",
		},
		Fixture:      fixture.NormalMode,
		SoftwareDeps: []string{"gsc"},
		VarDeps:      []string{"servo"},
		Params: []testing.Param{{
			Name: "batterybypasspp_enabled_and_battery_disconnected",
			Val: cCDCapabilitiesBatteryBypassPPParam{
				isBatteryBypassPPEnabled:               true,
				isBatteryConnected:                     false,
				shouldCCDOpenRequirePhysicalPresence:   false,
				shouldCCDUnlockRequirePhysicalPresence: false,
				shouldGsctoolRequirePhysicalPresence:   false,
			},
		}, {
			Name: "batterybypasspp_enabled_and_battery_connected",
			Val: cCDCapabilitiesBatteryBypassPPParam{
				isBatteryBypassPPEnabled:               true,
				isBatteryConnected:                     true,
				shouldCCDOpenRequirePhysicalPresence:   true,
				shouldCCDUnlockRequirePhysicalPresence: true,
				shouldGsctoolRequirePhysicalPresence:   true,
			},
		}, {
			Name: "batterybypasspp_disabled_and_battery_disconnected",
			Val: cCDCapabilitiesBatteryBypassPPParam{
				isBatteryBypassPPEnabled:               false,
				isBatteryConnected:                     false,
				shouldCCDOpenRequirePhysicalPresence:   true,
				shouldCCDUnlockRequirePhysicalPresence: true,
				shouldGsctoolRequirePhysicalPresence:   true,
			},
		}},
	})
}

func CCDCapabilitiesBatteryBypassPP(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper
	param := s.Param().(cCDCapabilitiesBatteryBypassPPParam)

	err := h.OpenCCD(ctx, true, false, servo.Lock)
	if err != nil {
		s.Fatal("Failed to get open CCD: ", err)
	}

	// Set BatteryBypassPP accordingly
	capState := servo.CapIfOpened
	if param.isBatteryBypassPPEnabled {
		capState = servo.CapAlways
	}
	err = h.Servo.SetCCDCapability(ctx, servo.BatteryBypassPP, capState)
	if err != nil {
		s.Fatal("Failed to set BatteryBypassPP capability to IfOpened", err)
	}

	// Set forced battery presence
	err = h.SetForceBatteryPresence(ctx, param.isBatteryConnected)
	if err != nil {
		s.Fatal("Failed to set forced battery presence", err)
	}

	err = h.LockCCD(ctx)
	if err != nil {
		s.Fatal("Failed to lock CCD", err)
	}

	// Check `ccd open` for physical presence
	err = h.AssertPhysicalPresenceRequiredForCCDOpen(ctx, param.shouldCCDOpenRequirePhysicalPresence)
	if err != nil {
		s.Fatal("Failed verify physical presence for CCD open: ", err)
	}

	// Check `ccd unlock` for physical presence
	err = h.AssertPhysicalPresenceRequiredForCCDUnlock(ctx, param.shouldCCDUnlockRequirePhysicalPresence)
	if err != nil {
		s.Fatal("Failed verify physical presence for CCD unlock: ", err)
	}

	// Check `gsctool -aF disable` for physical presence
	is_required, err := h.IsPhysicalPresenceRequiredForGsctool(ctx)
	if err != nil {
		s.Fatal("Failed verify physical presence for `gsctool -aF disable`:", err)
	}
	if param.shouldGsctoolRequirePhysicalPresence != is_required {
		s.Fatal("gsctool physical presence mismatch")
	}
}
