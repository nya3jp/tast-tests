// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"time"

	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/testing/hwdep"

	"chromiumos/tast/testing"

	"github.com/golang/protobuf/ptypes/empty"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         FWConsecutiveLidSwitch,
		Desc:         "Trigger lid switch on and off many times consecutively",
		Contacts:     []string{"js@semihalf.com", "chromeos-firmware@google.com"},
		Attr:         []string{"group:firmware", "firmware_unstable"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
		ServiceDeps:  []string{"tast.cros.firmware.UtilsService"},
		Fixture:      fixture.NormalMode,
		// This test could take a long time, at least 1 minute per retry
		Timeout: 40 * time.Minute,
	})
}

// FWConsecutiveLidSwitch triggers lid state on and off via Servo
// after logging to Chrome as Guest and then checks if boot ID is
// the same as before
func FWConsecutiveLidSwitch(ctx context.Context, s *testing.State) {

	const (
		testRetries int           = 100
		lidDelay    time.Duration = 2 * time.Second
		wakeDelay   time.Duration = 10 * time.Second
	)

	h := s.FixtValue().(*fixture.Value).Helper

	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to init servo: ", err)
	}

	if err := h.RequireConfig(ctx); err != nil {
		s.Fatal("Failed to get config: ", err)
	}

	if err := h.RequireRPCClient(ctx); err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}

	if err := h.RequireRPCUtils(ctx); err != nil {
		s.Fatal("Failed to require RPC utils: ", err)
	}

	// Need to login, because on the login screen the DUT goes off with
	// the lid closed, whereas the intent is to exercise suspend.
	s.Log("Creating new Chrome instance")
	if _, err := h.RPCUtils.NewChrome(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to create Chrome instance: ", err)
	}

	defer func() {
		s.Log("Closing Chrome instance")
		if _, err := h.RPCUtils.CloseChrome(ctx, &empty.Empty{}); err != nil {
			s.Fatal("Failed to close Chrome instance: ", err)
		}
	}()

	s.Log("Opening lid for the first time to be sure")
	if err := h.Servo.OpenLid(ctx); err != nil {
		s.Fatal("Failed to open lid: ", err)
	}

	initialBootID, err := h.Reporter.BootID(ctx)
	if err != nil {
		s.Fatal("Failed to acquire current boot ID: ", err)
	}
	s.Log("Current boot ID: ", initialBootID)

	for r := 0; r <= testRetries; r++ {
		s.Logf("Consecutive lid switch %d/%d", r, testRetries)

		if err := h.Servo.CloseLid(ctx); err != nil {
			s.Fatal("Failed to close lid: ", err)
		}

		if err := testing.Sleep(ctx, lidDelay); err != nil {
			s.Fatal("Failed to sleep during closed lid delay: ", err)
		}

		if err := h.DUT.WaitUnreachable(ctx); err != nil {
			s.Fatal("Failed to make DUT unreachable: ", err)
		}

		if err := h.Servo.OpenLid(ctx); err != nil {
			s.Fatal("Failed to open lid: ", err)
		}

		if err := testing.Sleep(ctx, wakeDelay); err != nil {
			s.Fatal("Failed to sleep during wake delay: ", err)
		}

		if err := h.DUT.WaitConnect(ctx); err != nil {
			s.Fatal("Failed to connect to DUT: ", err)
		}

		if err := testing.Sleep(ctx, lidDelay); err != nil {
			s.Fatal("Failed to sleep during open lid delay: ", err)
		}

		afterLidBootID, err := h.Reporter.BootID(ctx)
		if err != nil {
			s.Fatal("Failed to acquire boot ID after lid switch: ", err)
		}
		if afterLidBootID != initialBootID {
			s.Fatalf("Initial boot ID does not match boot ID after lid switch: got %s; want %s", afterLidBootID, initialBootID)
		}
	}
}
