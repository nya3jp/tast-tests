// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"time"

	fwCommon "chromiumos/tast/common/firmware"
	"chromiumos/tast/common/servo"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCDAccessibility,
		Desc:         "Verifies if we can open CCD while having capabilities in different states",
		Contacts:     []string{"cienet-firmware@cienet.corp-partner.google.com", "chromeos-firmware@google.com"},
		Attr:         []string{"group:firmware", "firmware_trial"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC(), hwdep.Battery()),
		Fixture:      fixture.NormalMode,
		Timeout:      60 * time.Minute, // Long timeout to account for the long PP sequence.
	})
}

type capSetup struct {
	capability servo.CCDCap
	state      servo.CCDCapState
}

func CCDAccessibility(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper

	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to init servo: ", err)
	}

	if err := h.RequireConfig(ctx); err != nil {
		s.Fatal("Failed to get config: ", err)
	}

	// For debugging purposes, log servo and dut connection type.
	servoType, err := h.Servo.GetServoType(ctx)
	if err != nil {
		s.Fatal("Failed to find servo type: ", err)
	}
	s.Logf("Servo type: %s", servoType)

	dutConnType, err := h.Servo.GetDUTConnectionType(ctx)
	if err != nil {
		s.Fatal("Failed to find dut connection type: ", err)
	}
	s.Logf("DUT connection type: %s", dutConnType)

	// Record the initial mode.
	initMode, err := h.Reporter.CurrentBootMode(ctx)
	if err != nil {
		s.Fatal("Failed to check boot mode: ", err)
	}

	s.Log("Ensure CCD is open, testlab enabled and capabilities set to factory mode")
	if err := h.OpenCCD(ctx, true, true); err != nil {
		s.Fatal("Failed to set CCD: ", err)
	}

	// Give enough time for the deferred function to restore DUT in case long PP is required to open CCD.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 9*time.Minute)
	defer cancel()

	defer func(ctx context.Context, initMode fwCommon.BootMode) {
		s.Log("Ensuring CCD is open at the end of the test")
		// Aim to leave the DUT with CCD open and capabilitites set to factory.
		if err := h.OpenCCD(ctx, true, true); err != nil {
			s.Fatal("Failed setting CCD at the end of the test: ", err)
		}
	}(cleanupCtx, initMode)

	// Verify if there is a micro-servo connected.
	hasMicroOrC2D2, err := h.Servo.PreferDebugHeader(ctx)
	if err != nil {
		s.Fatal("PreferDebugHeader: ", err)
	}

	for _, tc := range []struct {
		openNoLongPP  servo.CCDCapState
		openNoDevMode servo.CCDCapState
		openNoTPMWipe servo.CCDCapState
		openFromUSB   servo.CCDCapState
	}{
		{"Always", "IfOpened", "IfOpened", "IfOpened"},
		{"Always", "IfOpened", "IfOpened", "Always"},
		{"Always", "IfOpened", "Always", "IfOpened"},
		{"Always", "IfOpened", "Always", "Always"},
		{"Always", "Always", "IfOpened", "IfOpened"},
		{"Always", "Always", "IfOpened", "Always"},
		{"Always", "Always", "Always", "IfOpened"},
		{"Always", "Always", "Always", "Always"},
		{"IfOpened", "Always", "Always", "Always"},
	} {
		ccdSettings := map[servo.CCDCap]servo.CCDCapState{
			"OpenNoLongPP":  tc.openNoLongPP,
			"OpenNoDevMode": tc.openNoDevMode,
			"OpenNoTPMWipe": tc.openNoTPMWipe,
			"OpenFromUSB":   tc.openFromUSB,
		}
		s.Log("------------- Testing Parameters -------------")
		for capability, state := range ccdSettings {
			s.Logf("%s : %s", capability, state)
		}

		// Set OpenNoLongPP to IfOpened, and run the test on
		// opening CCD iff servo micro is present, which is
		// required in order for power presses to be recognized
		// by the Cr50.
		if ccdSettings["OpenNoLongPP"] == "IfOpened" && !hasMicroOrC2D2 {
			s.Log("Test skipped, no servo-micro or C2D2 found")
			continue
		}

		// Set capabilities to the desired state.
		if err := h.Servo.SetCCDCapability(ctx, ccdSettings); err != nil {
			s.Fatal("Failed setting CCD capability: ", err)
		}

		s.Log("----------------- Start Test -----------------")
		// Lock and Unlock CCD.
		if err := lockOpenCCDprocedure(ctx, h); err != nil {
			s.Fatalf("Failed while testing: %v: %s", tc, err)
		}
	}

	// Verify DUT ends up with the initial mode.
	mode, err := h.Reporter.CurrentBootMode(ctx)
	if err != nil {
		s.Fatal("Failed to check boot mode at the end of the test: ", err)
	}
	if mode != initMode {
		s.Fatalf("DUT mode is %q, but expected %q", mode, initMode)
	}
}

// lockOpenCCDprocedure will lock ccd and attempt to open it without using testlab.
func lockOpenCCDprocedure(ctx context.Context, h *firmware.Helper) error {
	// Lock CCD.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		re := `CCD locked.`
		_, err := h.Servo.RunCR50CommandGetOutput(ctx, "ccd lock", []string{re})
		if err != nil {
			return errors.Wrap(err, "failed to lock CCD")
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second, Interval: 2 * time.Second}); err != nil {
		return err
	}
	testing.ContextLog(ctx, "CCD is lock")

	// Attempt to open CCD without using testlab.
	if err := h.OpenCCDNoTestlab(ctx); err != nil {
		return errors.Wrap(err, "failed while OpenCCDNoTestlab")
	}
	return nil
}
