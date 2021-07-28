// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/network/cellular"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ShillCellularRoaming,
		Desc:     "Verifies that AllowRoaming is respected by Shill",
		Contacts: []string{"pholla@google.com", "cros-network-health@google.com"},
		Attr:     []string{"group:cellular", "cellular_unstable"},
		Fixture:  "cellular",
		Timeout:  60 * time.Second,
	})
}

func ShillCellularRoaming(ctx context.Context, s *testing.State) {
	helper, err := cellular.NewHelper(ctx)
	if err != nil {
		s.Fatal("Failed to create cellular.Helper: ", err)
	}

	// Disable AutoConnect so that we can explicitly control connection state.
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

	service, err := helper.FindServiceForDevice(ctx)
	if err != nil {
		s.Fatalf("Could not find default service for device: %s", err)
	}

	// Check that we have a roaming sim, else exit early
	if err := service.WaitForProperty(ctx, shillconst.ServicePropertyCellularRoamingState, "roaming", shillconst.DefaultTimeout); err != nil {
		s.Fatalf("Could not check if a roaming sim is inserted: %s", err)
	}

	isConnected, err := service.IsConnected(ctx)
	if err != nil {
		s.Fatalf("Could not check if service is connected: %s", err)
	}
	if isConnected {
		if err := service.Disconnect(ctx); err != nil {
			s.Fatalf("Failed to disconnect from roaming network prior to starting the actual test: %s", err)
		}
	}

	// Set AllowRoaming to true at a device level, in order to test Service.AllowRoaming.
	// Roaming is allowed when both Device.AllowRoaming and Service.AllowRoaming are true.
	s.Log("Set Device.PolicyAllowRoaming = true")
	if err := helper.Device.SetProperty(ctx, shillconst.DevicePropertyCellularPolicyAllowRoaming, true); err != nil {
		s.Fatalf("Could not set PolicyAllowRoaming to true: %s", err)
	}

	s.Log("Set Service.AllowRoaming = false")
	if err := service.SetProperty(ctx, shillconst.ServicePropertyCellularAllowRoaming, false); err != nil {
		s.Fatalf("Could not set AllowRoaming property to false: %s", err)
	}

	s.Log("Connect to roaming network when Service.AllowRoaming = false, expecting connect to fail")
	if err := service.Connect(ctx); err == nil {
		s.Fatal("Able to connect to roaming service despite Service.AllowRoaming = false")
	}

	s.Log("Set Service.AllowRoaming = true")
	if err := service.SetProperty(ctx, shillconst.ServicePropertyCellularAllowRoaming, true); err != nil {
		s.Fatalf("Could not set AllowRoaming property to true: %s", err)
	}

	s.Log("Connect to roaming network when Service.AllowRoaming = True, expecting connect to succeed")
	if err := helper.ConnectToService(ctx, service); err != nil {
		s.Fatalf("Unable to connect to roaming service %s", err)
	}

	s.Log("Set Service.AllowRoaming = false, expecting disconnection from roaming network")
	if err := service.SetProperty(ctx, shillconst.ServicePropertyCellularAllowRoaming, false); err != nil {
		s.Fatalf("Could not set AllowRoaming property to false: %s", err)
	}

	if err := service.WaitForProperty(ctx, shillconst.ServicePropertyIsConnected, false, shillconst.DefaultTimeout); err != nil {
		s.Fatalf("Service is connected  to a roaming network when AllowRoaming = false: %s", err)
	}

}
