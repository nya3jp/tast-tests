// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"time"

	fwCommon "chromiumos/tast/common/firmware"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/pre"
	"chromiumos/tast/remote/servo"
	"chromiumos/tast/testing"
)

// bootModeTestParams defines the params for a single test-case.
// bootToMode defines which boot-mode to switch the DUT into.
// allowGBBForce defines whether to force dev mode via GBB flags.
// resetAfterBoot defines whether to perform a ModeAwareReboot after switching to bootToMode.
// resetType defines whether ModeAwareReboot should use a warm or a cold reset.
type bootModeTestParams struct {
	bootToMode     fwCommon.BootMode
	allowGBBForce  bool
	resetAfterBoot bool
	resetType      firmware.ResetType
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         BootMode,
		Desc:         "Verifies that remote tests can boot the DUT into, and confirm that the DUT is in, the different firmware modes (normal, dev, and recovery)",
		Contacts:     []string{"cros-fw-engprod@google.com"},
		Data:         []string{firmware.ConfigFile},
		ServiceDeps:  []string{"tast.cros.firmware.UtilsService", "tast.cros.firmware.BiosService"},
		SoftwareDeps: []string{"crossystem", "flashrom"},
		Attr:         []string{"group:firmware"},
		Timeout:      15 * time.Minute, // Enough time to download test image from CloudStorage if necessary
		Params: []testing.Param{{
			Name: "normal",
			Pre:  pre.NormalMode(),
			Val: bootModeTestParams{
				bootToMode: fwCommon.BootModeNormal,
			},
			ExtraAttr: []string{"firmware_smoke"},
		}, {
			Name: "normal_warm",
			Pre:  pre.NormalMode(),
			Val: bootModeTestParams{
				bootToMode:     fwCommon.BootModeNormal,
				resetAfterBoot: true,
				resetType:      firmware.WarmReset,
			},
			ExtraAttr: []string{"firmware_smoke"},
		}, {
			Name: "normal_cold",
			Pre:  pre.NormalMode(),
			Val: bootModeTestParams{
				bootToMode:     fwCommon.BootModeNormal,
				resetAfterBoot: true,
				resetType:      firmware.ColdReset,
			},
			ExtraAttr: []string{"firmware_smoke"},
		}, {
			Name: "rec",
			Pre:  pre.NormalMode(),
			Val: bootModeTestParams{
				bootToMode: fwCommon.BootModeRecovery,
			},
			ExtraAttr: []string{"firmware_smoke", "firmware_usb"},
		}, {
			Name: "rec_warm",
			Pre:  pre.NormalMode(),
			Val: bootModeTestParams{
				bootToMode:     fwCommon.BootModeRecovery,
				resetAfterBoot: true,
				resetType:      firmware.WarmReset,
			},
			ExtraAttr: []string{"firmware_smoke", "firmware_usb"},
		}, {
			Name: "rec_cold",
			Pre:  pre.NormalMode(),
			Val: bootModeTestParams{
				bootToMode:     fwCommon.BootModeRecovery,
				resetAfterBoot: true,
				resetType:      firmware.ColdReset,
			},
			ExtraAttr: []string{"firmware_smoke", "firmware_usb"},
		}, {
			Name: "dev",
			Pre:  pre.NormalMode(),
			Val: bootModeTestParams{
				bootToMode: fwCommon.BootModeDev,
			},
			ExtraAttr: []string{"firmware_experimental"},
		}, {
			Name: "dev_warm",
			Pre:  pre.NormalMode(),
			Val: bootModeTestParams{
				bootToMode:     fwCommon.BootModeDev,
				resetAfterBoot: true,
				resetType:      firmware.WarmReset,
			},
			ExtraAttr: []string{"firmware_experimental"},
		}, {
			Name: "dev_cold",
			Pre:  pre.NormalMode(),
			Val: bootModeTestParams{
				bootToMode:     fwCommon.BootModeDev,
				resetAfterBoot: true,
				resetType:      firmware.ColdReset,
			},
			ExtraAttr: []string{"firmware_experimental"},
		}, {
			Name: "dev_to_rec",
			Pre:  pre.DevMode(),
			Val: bootModeTestParams{
				bootToMode: fwCommon.BootModeRecovery,
			},
			ExtraAttr: []string{"firmware_smoke", "firmware_usb"},
		}, {
			Name: "rec_to_dev",
			Pre:  pre.RecMode(),
			Val: bootModeTestParams{
				bootToMode: fwCommon.BootModeDev,
			},
			ExtraAttr: []string{"firmware_experimental", "firmware_usb"},
		}, {
			Name: "dev_gbb",
			Pre:  pre.NormalMode(),
			Val: bootModeTestParams{
				bootToMode:    fwCommon.BootModeDev,
				allowGBBForce: true,
			},
			ExtraAttr: []string{"firmware_experimental"},
		}, {
			Name: "dev_gbb_to_rec",
			Pre:  pre.DevModeGBB(),
			Val: bootModeTestParams{
				bootToMode: fwCommon.BootModeRecovery,
			},
			ExtraAttr: []string{"firmware_experimental", "firmware_usb"},
		}, {
			Name: "rec_to_dev_gbb",
			Pre:  pre.RecMode(),
			Val: bootModeTestParams{
				bootToMode:    fwCommon.BootModeDev,
				allowGBBForce: true,
			},
			ExtraAttr: []string{"firmware_experimental", "firmware_usb"},
		}, {
			Name: "dev_gbb_to_normal",
			Pre:  pre.DevModeGBB(),
			Val: bootModeTestParams{
				bootToMode: fwCommon.BootModeNormal,
			},
			ExtraAttr: []string{"firmware_experimental"},
		}},
		Vars: []string{"servo"},
	})
}

func BootMode(ctx context.Context, s *testing.State) {
	tc := s.Param().(bootModeTestParams)
	pv := s.PreValue().(*pre.Value)
	h := pv.Helper
	ms, err := firmware.NewModeSwitcher(ctx, h)
	if err != nil {
		s.Fatal("Creating mode switcher: ", err)
	}

	// Report ModeSwitcherType, for debugging.
	if err := h.RequireConfig(ctx); err != nil {
		s.Fatal("Requiring config")
	}
	s.Log("Mode switcher type: ", h.Config.ModeSwitcherType)

	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Error opening servo: ", err)
	}
	if tc.bootToMode == fwCommon.BootModeRecovery {
		if err := h.SetupUSBKey(ctx, s.CloudStorage()); err != nil {
			s.Fatal("USBKey not working: ", err)
		}
	}

	// Double-check that DUT starts in the right mode.
	if curr, err := h.Reporter.CurrentBootMode(ctx); err != nil {
		s.Fatal("Checking boot mode at beginning of test: ", err)
	} else if curr != pv.BootMode {
		s.Logf("DUT started in boot mode %s. Setting up %s", curr, pv.BootMode)
		if err = ms.RebootToMode(ctx, pv.BootMode); err != nil {
			s.Fatalf("Failed to set up %s mode: %s", pv.BootMode, err)
		}
	}

	// Sometimes the test leaves the DUT powered-off, which prevents other tests from running.
	// To prevent this, defer a cleanup function to reset the DUT if unconnected.
	ctxForCleanup := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Minute)
	defer cancel()
	defer func(ctx context.Context) {
		if h.DUT.Connected(ctx) {
			return
		}
		s.Log("DUT not connected at end-of-test. Cold-resetting")
		if err := h.RequireServo(ctx); err != nil {
			s.Fatal("Requiring servo during cleanup: ", err)
		}
		if err := h.Servo.SetPowerState(ctx, servo.PowerStateReset); err != nil {
			s.Fatal("Resetting DUT during cleanup: ", err)
		}
		if err := h.DUT.WaitConnect(ctx); err != nil {
			s.Fatal("Reconnecting to DUT during cleanup: ", err)
		}
	}(ctxForCleanup)

	// Switch to tc.bootToMode.
	// RebootToMode ensures that the DUT winds up in the expected boot mode afterward.
	var opts []firmware.ModeSwitchOption
	if tc.allowGBBForce {
		opts = append(opts, firmware.AllowGBBForce)
	} else if !pv.ForcesDevMode() {
		// Don't check the dev-force GBB flag if there's no reason for it to have been set.
		opts = append(opts, firmware.AssumeGBBFlagsCorrect)
	}
	s.Logf("Transitioning to %s mode with options %+v", tc.bootToMode, opts)
	if err = ms.RebootToMode(ctx, tc.bootToMode, opts...); err != nil {
		s.Fatalf("Error during transition from %s to %s: %+v", fwCommon.BootModeNormal, tc.bootToMode, err)
	}
	s.Log("Transition completed successfully")

	// Reset the DUT, if the test case calls for it.
	// ModeAwareReboot ensures the DUT winds up in the expected boot mode afterward.
	if tc.resetAfterBoot {
		s.Logf("Resetting DUT (resetType=%v)", tc.resetType)
		if err := ms.ModeAwareReboot(ctx, tc.resetType); err != nil {
			s.Fatal("Error resetting DUT: ", err)
		}
		s.Log("Reset completed successfully")
	}

	// Switch back to normal mode.
	// This isn't necessary if the DUT is already in normal mode.
	if curr, err := h.Reporter.CurrentBootMode(ctx); err != nil {
		s.Fatal("Failed to determine DUT boot mode: ", err)
	} else if curr != fwCommon.BootModeNormal {
		s.Logf("Transitioning back from %s to normal mode", curr)
		if err = ms.RebootToMode(ctx, fwCommon.BootModeNormal); err != nil {
			s.Fatalf("Error returning from %s to %s: %+v", curr, fwCommon.BootModeNormal, err)
		}
		s.Log("Transition completed successfully")
	}
}
