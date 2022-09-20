// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"math/rand"
	"strconv"
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
		Vars:         []string{"cellular.ShillCellularSafetyDance.seed"},
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

	// Run setup random sequence
	var seed int64
	if seedStr, ok := s.Var("cellular.ShillCellularSafetyDance.seed"); ok {
		val, err := strconv.ParseInt(seedStr, 10, 64)
		if err != nil {
			s.Fatalf("Invalid seed value given: %s", seedStr)
		}
		seed = val
	} else {
		seed = time.Now().UnixNano()
	}
	rsource := rand.NewSource(seed)
	s.Logf("Running test with seed: %d", seed)

	// Define the list of possible states and which are the allowed actions in each
	const (
		Disabled     int = 0
		Enabled          = 1
		Connected        = 2
		Disconnected     = 3
	)
	nextStates := [][]int{
		Disabled:     {Enabled},
		Enabled:      {Disabled, Connected},
		Connected:    {Disabled, Disconnected},
		Disconnected: {Disabled, Connected},
	}
	actionList := []func(ctx context.Context, s *testing.State, helper *cellular.Helper, i int){
		Disabled:     actionDisable,
		Enabled:      actionEnable,
		Connected:    actionConnect,
		Disconnected: actionDisconnect,
	}

	// Start disabled
	if _, err := helper.Disable(ctx); err != nil {
		s.Fatalf("Initial disable failed: %s", err)
	}
	currentState := Disabled

	// Run N random actions, limited to the ones available in each step
	const nTests = 30
	for i := 0; i < nTests; i++ {
		nextState := nextStates[currentState][rand.New(rsource).Intn(len(nextStates[currentState]))]
		actionList[nextState](ctx, s, helper, i)
		currentState = nextState
	}

	// Finish enabled
	if _, err := helper.Enable(ctx); err != nil {
		s.Fatalf("Final enable failed: %s", err)
	}
}

func actionDisable(ctx context.Context, s *testing.State, helper *cellular.Helper, i int) {
	s.Logf("[%d] ACTION: Disable", i)
	if _, err := helper.Disable(ctx); err != nil {
		s.Fatalf("[%d]   Disable failed: %s", i, err)
	}

	serviceCtx, serviceCancel := context.WithTimeout(ctx, 3*time.Second)
	defer serviceCancel()
	if _, err := helper.FindServiceForDevice(serviceCtx); err == nil {
		s.Fatalf("[%d]   Service found while Disabled", i)
	}
	s.Logf("[%d]   Disabled", i)
}

func actionEnable(ctx context.Context, s *testing.State, helper *cellular.Helper, i int) {
	s.Logf("[%d] ACTION: Enable", i)
	if _, err := helper.Enable(ctx); err != nil {
		s.Fatalf("[%d]   Enable failed: %s", i, err)
	}

	serviceCtx, serviceCancel := context.WithTimeout(ctx, 3*time.Second)
	defer serviceCancel()
	if _, err := helper.FindServiceForDevice(serviceCtx); err != nil {
		s.Fatalf("[%d]   Service not found while Enabled: %s", i, err)
	}
	s.Logf("[%d]   Enabled", i)
}

func actionConnect(ctx context.Context, s *testing.State, helper *cellular.Helper, i int) {
	s.Logf("[%d] ACTION: Connect", i)
	if _, err := helper.ConnectToDefault(ctx); err != nil {
		s.Fatalf("[%d]   Connect failed: %s", i, err)
	}
	s.Logf("[%d]   Connected", i)
}

func actionDisconnect(ctx context.Context, s *testing.State, helper *cellular.Helper, i int) {
	s.Logf("[%d] ACTION: Disconnect", i)
	if _, err := helper.Disconnect(ctx); err != nil {
		s.Fatalf("[%d]   Disconnect failed: %s", i, err)
	}
	s.Logf("[%d]   Disconnected", i)
}
