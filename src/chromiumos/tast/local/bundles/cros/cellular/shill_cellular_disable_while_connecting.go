// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"sync"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/cellular"
	"chromiumos/tast/local/modemmanager"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ShillCellularDisableWhileConnecting,
		Desc:         "Verifies that the modem can handle being disabled while connecting in Shill",
		Contacts:     []string{"ejcaruso@google.com", "chromeos-cellular-team@google.com"},
		Attr:         []string{"group:cellular", "cellular_unstable", "cellular_sim_active"},
		HardwareDeps: hwdep.D(hwdep.Cellular()),
		Timeout:      3 * time.Minute,
		Fixture:      "cellular",
	})
}

func ShillCellularDisableWhileConnecting(ctx context.Context, s *testing.State) {
	if _, err := modemmanager.NewModemWithSim(ctx); err != nil {
		s.Fatal("Could not find MM dbus object with a valid sim: ", err)
	}

	helper, err := cellular.NewHelper(ctx)
	if err != nil {
		s.Fatal("Failed to create cellular.Helper: ", err)
	}

	// Disable AutoConnect so that enable does not connect.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 20*time.Second)
	defer cancel()
	if wasAutoConnect, err := helper.SetServiceAutoConnect(ctx, false); err != nil {
		s.Fatal("Failed to disable AutoConnect: ", err)
	} else if wasAutoConnect {
		defer func(ctx context.Context) {
			if _, err := helper.SetServiceAutoConnect(ctx, true); err != nil {
				s.Fatal("Failed to enable AutoConnect: ", err)
			}
		}(cleanupCtx)
	}

	// Simply disconnecting the service here causes the later connection attempt to
	// succeed too quickly for the test to catch it with the disable. Instead, disable
	// and enable the device.
	s.Log("Toggling device")
	if _, err := helper.Disable(ctx); err != nil {
		s.Fatal("Failed to disable device: ", err)
	}
	if _, err := helper.Enable(ctx); err != nil {
		s.Fatal("Failed to enable device: ", err)
	}

	defer func(ctx context.Context) {
		if _, err := helper.Enable(ctx); err != nil {
			s.Fatal("Failed to re-enable device: ", err)
		}
	}(cleanupCtx)

	service, err := helper.FindServiceForDevice(ctx)
	if err != nil {
		s.Fatal("Could not find service for device: ", err)
	}

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()

		// Wait for service to move to the "Associating" or "Configuring" state. This means
		// the service has started connecting.
		s.Log("Waiting for service to start connecting")
		states := []interface{}{shillconst.ServiceStateAssociation, shillconst.ServiceStateConfiguration}
		if err := service.WaitForPropertyInSetWithOptions(ctx, shillconst.ServicePropertyState, states, &testing.PollOptions{Timeout: 10 * time.Second, Interval: 20 * time.Millisecond}); err != nil {
			s.Fatal("Service never started connecting: ", err)
		}

		// Now we can disable the device.
		s.Log("Disabling device")
		if _, err := helper.Disable(ctx); err != nil {
			s.Fatal("Failed to disable the device: ", err)
		}
	}()

	s.Log("Connecting service")
	if err := helper.ConnectToServiceWithTimeout(ctx, service, time.Minute); err != nil {
		s.Log("Connection failed: ", err)
	} else {
		s.Log("Connection succeeded")
	}

	s.Log("Waiting for disable goroutine")
	wg.Wait()
}
