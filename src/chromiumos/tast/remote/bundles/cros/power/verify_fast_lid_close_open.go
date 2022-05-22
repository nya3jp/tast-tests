// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/remote/powercontrol"
	"chromiumos/tast/testing"
)

const ecErrorMsg string = "CPU did not enter SLP S0 for suspend-to-idle"

func init() {
	testing.AddTest(&testing.Test{
		Func:         VerifyFastLidCloseOpen,
		Desc:         "To verify Fast lid close open multiple times",
		Vars:         []string{"power.iterations"},
		SoftwareDeps: []string{"chrome"},
		LacrosStatus: testing.LacrosVariantUnneeded,
		ServiceDeps:  []string{"tast.cros.security.BootLockboxService"},
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		Fixture:      fixture.NormalMode,
	})
}

func VerifyFastLidCloseOpen(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	h := s.FixtValue().(*fixture.Value).Helper

	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}

	// Perform a Chrome login.
	s.Log("Login to Chrome")
	if err := powercontrol.ChromeOSLogin(ctx, s.DUT(), s.RPCHint()); err != nil {
		s.Fatal("Failed to login to chrome: ", err)
	}
	iteration := 1
	if val, ok := s.Var("power.iterations"); ok {
		i, err := strconv.Atoi(val)
		if err != nil {
			s.Fatal("Failed to convert var to int: ", err)
		}
		iteration = i
	}

	s.Log("Capturing EC log")
	if err := h.Servo.SetOnOff(ctx, servo.ECUARTCapture, servo.On); err != nil {
		s.Fatal("Failed to capture EC UART: ", err)
	}
	defer h.Servo.SetOnOff(cleanupCtx, servo.ECUARTCapture, servo.Off)

	// Read the uart stream just to make sure there isn't buffered data.
	if _, err := h.Servo.GetQuotedString(ctx, servo.ECUARTStream); err != nil {
		s.Fatal("Failed to read UART: ", err)
	}

	for i := 1; i <= iteration; i++ {
		s.Logf("Iteration: %d/%d", i, iteration)

		// Emulate DUT lid closing.
		if err := h.Servo.CloseLid(ctx); err != nil {
			s.Fatal("Failed to close DUT's lid: ", err)
		}
		s.Log("Checking lid state after closing lid")
		lidState, err := h.Servo.LidOpenState(ctx)
		if err != nil {
			s.Fatal("Failed to check the final lid state: ", err)
		}
		if lidState != string(servo.LidOpenNo) {
			s.Fatalf("Failed to check DUT lid state, expected: %q got: %q", servo.LidOpenNo, lidState)
		}

		delay := 10 * time.Second
		s.Logf("Waiting for %v after closing DUT's lid", delay)
		if err := testing.Sleep(ctx, delay); err != nil {
			s.Fatal("Failed to sleep after closing DUT's lid: ", err)
		}

		// Emulate DUT lid opening.
		if err := h.Servo.OpenLid(ctx); err != nil {
			s.Fatal("Failed to open DUT's lid: ", err)
		}
		s.Log("Checking lid state after opening lid")
		lidState, err = h.Servo.LidOpenState(ctx)
		if err != nil {
			s.Fatal("Failed to check the final lid state: ", err)
		}
		if lidState != string(servo.LidOpenYes) {
			s.Fatalf("Failed to check DUT lid state, expected: %q got: %q", servo.LidOpenYes, lidState)
		}

		s.Logf("Waiting for %v after opening DUT's lid", delay)
		if err := testing.Sleep(ctx, delay); err != nil {
			s.Fatal("Failed to sleep after opening DUT's lid: ", err)
		}

		if lines, err := h.Servo.GetQuotedString(ctx, servo.ECUARTStream); err != nil {
			s.Fatal("Failed to read UART: ", err)
		} else if lines != "" {
			for _, l := range strings.Split(lines, "\r\n") {
				if strings.Contains(l, ecErrorMsg) {
					s.Error("Failed to verify EC logs, Errors found in EC logs for Lid close open")
				}
			}
		}
	}
	defer func(ctx context.Context) {
		// Opening lid incase the test fails in the middle.
		if err := h.Servo.OpenLid(ctx); err != nil {
			s.Fatal("Failed to open DUT's lid: ", err)
		}
	}(cleanupCtx)
}
