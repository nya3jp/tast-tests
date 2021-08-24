// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/common/mmconst"
	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/local/bundles/cros/network/cellular"
	"chromiumos/tast/local/modemmanager"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ShillCellularRoaming,
		Desc:     "Verifies that AllowRoaming is respected by Shill",
		Contacts: []string{"pholla@google.com", "cros-network-health@google.com"},
		Attr:     []string{"group:cellular", "cellular_unstable", "cellular_sim_roaming"},
		Fixture:  "cellular",
		Timeout:  60 * time.Second,
	})
}

func ShillCellularRoaming(ctx context.Context, s *testing.State) {
	modem, err := modemmanager.NewModemWithSim(ctx)
	if err != nil {
		s.Fatal("Could not find MM dbus object with a valid sim: ", err)
	}

	helper, err := cellular.NewHelper(ctx)
	if err != nil {
		s.Fatal("Failed to create cellular.Helper: ", err)
	}

	if err := helper.InitServiceProperty(ctx, s, shillconst.ServicePropertyAutoConnect, false); err!=nil {
		s.Fatal("Could not initialize autoconnect to false: ", err)
	}

	service, err := helper.FindServiceForDevice(ctx)
	if err != nil {
		s.Fatal("Could not find default service for device", err)
	}

	// Check that we have a roaming sim, else exit early
	if err := service.WaitForProperty(ctx, shillconst.ServicePropertyCellularRoamingState, "roaming", shillconst.DefaultTimeout); err != nil {
		s.Fatal("Could not check if a roaming sim is inserted", err)
	}

	isConnected, err := service.IsConnected(ctx)
	if err != nil {
		s.Fatal("Could not check if service is connected", err)
	}
	if isConnected {
		if err := service.Disconnect(ctx); err != nil {
			s.Fatal("Failed to disconnect from roaming network prior to starting the actual test", err)
		}
	}

	// Set AllowRoaming to true at a device level, in order to test Service.AllowRoaming.
	// Roaming is allowed when both Device.PolicyAllowRoaming and Service.AllowRoaming are true.
	s.Log("Set Device.PolicyAllowRoaming = true")
	if err := helper.InitDeviceProperty(ctx, s, shillconst.DevicePropertyCellularPolicyAllowRoaming, true); err != nil {
		s.Fatalf("Could not set PolicyAllowRoaming to true: %s", err)
	}

	s.Log("Set Service.AllowRoaming = true")
	if err := helper.InitServiceProperty(ctx, s, shillconst.ServicePropertyCellularAllowRoaming, true); err != nil {
		s.Fatalf("Could not set AllowRoaming property to true: %s", err)
	}

	if err := modem.WaitForState(ctx, mmconst.ModemStateRegistered, time.Minute); err!=nil {
		s.Fatal("Modem is not registered")
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
		s.Fatalf("Service is connected to a roaming network when AllowRoaming = false: %s", err)
	}

	if err := service.SetProperty(ctx, shillconst.ServicePropertyCellularAllowRoaming, false); err != nil {
		s.Fatalf("Could not set AllowRoaming property to false: %s", err)
	}

	if err := modem.WaitForState(ctx, mmconst.ModemStateRegistered, time.Minute); err!=nil {
		s.Fatal("Modem is not registered")
	}

	s.Log("Connect to roaming network when Service.AllowRoaming = false, expecting connect to fail")
	if err := service.Connect(ctx); err == nil {
		s.Fatal("Able to connect to roaming service despite Service.AllowRoaming = false")
	}

}
