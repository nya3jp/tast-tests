// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"math/rand"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/cellular"
	"chromiumos/tast/local/modemmanager"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ShillCellularSafetyDance,
		Desc:         "Stress tests enable/disable/connect/disconnect operations in the Cellular Service",
		Contacts:     []string{"aleksandermj@google.com", "chromeos-cellular-team@google.com"},
		Attr:         []string{"group:cellular", "cellular_unstable", "cellular_sim_active"},
		HardwareDeps: hwdep.D(hwdep.Cellular()),
		Timeout:      10 * time.Minute,
		Fixture:      "cellular",
	})
}

func ShillCellularSafetyDance(ctx context.Context, s *testing.State) {
	if _, err := modemmanager.NewModemWithSim(ctx); err != nil {
		s.Fatal("Could not find MM dbus object with a valid sim: ", err)
	}

	helper, err := cellular.NewHelper(ctx)
	if err != nil {
		s.Fatal("Failed to create cellular.Helper: ", err)
	}

	// Disable AutoConnect so that enable does not connect
	ctxForAutoConnectCleanUp := ctx
	ctx, cancel := ctxutil.Shorten(ctx, cellular.AutoConnectCleanupTime)
	defer cancel()
	if wasAutoConnect, err := helper.SetServiceAutoConnect(ctx, false); err != nil {
		s.Fatal("Failed to disable AutoConnect: ", err)
	} else if wasAutoConnect {
		defer func(ctx context.Context) {
			if _, err := helper.SetServiceAutoConnect(ctx, true); err != nil {
				s.Fatal("Failed to enable AutoConnect: ", err)
			}
		}(ctxForAutoConnectCleanUp)
	}

	// Start disabled
	if _, err := helper.Disable(ctx); err != nil {
		s.Fatalf("Initial disable failed: %s", err)
	}

	// Run random sequence of N operations
	seed := time.Now().UnixNano()
	rsource := rand.NewSource(seed)
	s.Logf("Running test with seed: %d", seed)

	// Run N random actions
	const nTests = 30
	flist := []func(ctx context.Context, s *testing.State, helper *cellular.Helper, i int){
		actionDisable,
		actionEnable,
		actionConnect,
		actionDisconnect,
	}
	for i := 0; i < nTests; i++ {
		r := rand.New(rsource)
		flist[r.Intn(len(flist))](ctx, s, helper, i)
	}

	// Finish enabled
	if _, err := helper.Enable(ctx); err != nil {
		s.Fatalf("Final enable failed: %s", err)
	}
}

func actionDisable(ctx context.Context, s *testing.State, helper *cellular.Helper, i int) {
	s.Logf("[%d] ACTION: Disable", i)

	s.Logf("[%d]   Disabling", i)
	if _, err := helper.Disable(ctx); err != nil {
		s.Fatalf("[%d]   Disable failed: %s", i, err)
	}

	s.Logf("[%d]   Ensure Cellular Service is disabled", i)
	const shortTimeout = 3 * time.Second
	if _, err := helper.FindServiceForDeviceWithTimeout(ctx, shortTimeout); err == nil {
		s.Fatalf("[%d]   Service found while Disabled", i)
	}
}

func actionEnable(ctx context.Context, s *testing.State, helper *cellular.Helper, i int) {
	s.Logf("[%d] ACTION: Enable", i)

	s.Logf("[%d]   Enabling", i)
	if _, err := helper.Enable(ctx); err != nil {
		s.Fatalf("[%d]   Enable failed: %s", i, err)
	}

	s.Logf("[%d]   Ensure Cellular Service is enabled", i)
	const shortTimeout = 3 * time.Second
	if _, err := helper.FindServiceForDeviceWithTimeout(ctx, shortTimeout); err != nil {
		s.Fatalf("[%d]   Service not found while Enabled: %s", i, err)
	}
}

func actionConnect(ctx context.Context, s *testing.State, helper *cellular.Helper, i int) {
	s.Logf("[%d] ACTION: Connect", i)

	s.Logf("[%d]   Checking if cellular service is available before attempting to connect", i)
	const shortTimeout = 3 * time.Second
	service, err := helper.FindServiceForDeviceWithTimeout(ctx, shortTimeout)
	if err != nil {
		s.Logf("[%d]   --- Invalid transition: skipping connect as cellular service is disabled", i)
		return
	}

	s.Logf("[%d]   Checking cellular service state before attempting to connect", i)
	state, err := service.GetState(ctx)
	if err != nil {
		s.Fatalf("[%d]   Failed to get cellular service state: %s", i, err)
		return
	}
	s.Logf("[%d]   Cellular service state: %s", i, state)

	if state != "idle" {
		s.Logf("[%d]   --- Invalid transition: skipping connect as cellular service is: %s", i, state)
		return
	}

	s.Logf("[%d]   Connecting", i)
	if _, err := helper.ConnectToDefault(ctx); err != nil {
		s.Fatalf("[%d]   Connect failed: %s", i, err)
	}
}

func actionDisconnect(ctx context.Context, s *testing.State, helper *cellular.Helper, i int) {
	s.Logf("[%d] ACTION: Disconnect", i)

	s.Logf("[%d]   Checking if cellular service is available before attempting to disconnect", i)
	const shortTimeout = 3 * time.Second
	service, err := helper.FindServiceForDeviceWithTimeout(ctx, shortTimeout)
	if err != nil {
		s.Logf("[%d]   --- Invalid transition: skipping disconnect as cellular service is disabled", i)
		return
	}

	s.Logf("[%d]   Checking cellular service state before attempting to disconnect", i)
	state, err := service.GetState(ctx)
	if err != nil {
		s.Fatalf("[%d]   Failed to get cellular service state: %s", i, err)
		return
	}
	s.Logf("[%d]   Cellular service state: %s", i, state)

	if state == "idle" {
		s.Logf("[%d]   --- Invalid transition: skipping disconnect as cellular service is idle", i)
		return
	}

	s.Logf("[%d]   Disconnecting", i)
	if _, err := helper.Disconnect(ctx); err != nil {
		s.Fatalf("[%d]   Disconnect failed: %s", i, err)
	}
}
