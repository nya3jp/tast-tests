// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/network/cellular"
	"chromiumos/tast/local/modemmanager"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

// Note: This test enables and connects to Cellular if not already enabled or connected.

func init() {
	testing.AddTest(&testing.Test{
		Func: ShillCellularSmoke,
		Desc: "Verifies that traffic can be sent over the Cellular network",
		Contacts: []string{
			"stevenjb@google.com",
			"chromeos-cellular-team@google.com",
		},
		Attr:    []string{"group:cellular", "cellular_unstable"},
		Fixture: "cellular",
		Timeout: 10 * time.Minute,
	})
}

func ShillCellularSmoke(ctx context.Context, s *testing.State) {
	if _, err := modemmanager.NewModemWithSim(ctx); err != nil {
		s.Fatal("Could not find MM dbus object with a valid sim: ", err)
	}

	helper, err := cellular.NewHelper(ctx)
	if err != nil {
		s.Fatal("Failed to create cellular.Helper: ", err)
	}

	// Disable Ethernet and/or WiFi if present and defer re-enabling.
	if enableFunc, err := helper.Manager.DisableTechnologyForTesting(ctx, shill.TechnologyEthernet); err != nil {
		s.Fatal("Unable to disable Ethernet: ", err)
	} else if enableFunc != nil {
		newCtx, cancel := ctxutil.Shorten(ctx, shill.EnableWaitTime)
		defer cancel()
		defer enableFunc(ctx)
		ctx = newCtx
	}
	if enableFunc, err := helper.Manager.DisableTechnologyForTesting(ctx, shill.TechnologyWifi); err != nil {
		s.Fatal("Unable to disable Wifi: ", err)
	} else if enableFunc != nil {
		newCtx, cancel := ctxutil.Shorten(ctx, shill.EnableWaitTime)
		defer cancel()
		defer enableFunc(ctx)
		ctx = newCtx
	}

	// Verify that a connectable Cellular service exists and ensure it is connected.
	service, err := helper.FindServiceForDevice(ctx)
	if err != nil {
		s.Fatal("Unable to find Cellular Service for Device: ", err)
	}
	if isConnected, err := service.IsConnected(ctx); err != nil {
		s.Fatal("Unable to get IsConnected for Service: ", err)
	} else if !isConnected {
		if err := helper.ConnectToDefault(ctx); err != nil {
			s.Fatal("Unable to Connect to Service: ", err)
		}
	}

	// This URL comes from src/third_party/autotest/files/client/cros/network.py.
	// Code for the app is here: https://chromereviews.googleplex.com/2390012/
	const hostName = "testing-chargen.appspot.com"

	// Verify that traffic will be sent over Cellular by getting the routing interface for hostName
	// and verifying that it matches the interface for the Cellular Device.
	addrs, err := net.LookupHost(hostName)
	if err != nil || len(addrs) < 1 {
		s.Fatalf("Unable to lookup address for host: %q: %s", hostName, err)
	}
	s.Log("IP: ", addrs[0])

	route, err := testexec.CommandContext(ctx, "ip", "route", "get", addrs[0]).Output()
	if err != nil {
		s.Fatal("Error running 'ip route': ", err)
	}
	// 'ip route get' returns strings in the format:
	// <addr> via <gateway> dev <interface>
	routeIface := strings.Split(string(route), " ")[4]

	props, err := helper.Device.GetProperties(ctx)
	if err != nil {
		s.Fatal("Error getting Device properties: ", err)
	}
	deviceIface, err := props.GetString(shillconst.DevicePropertyInterface)
	if err != nil {
		s.Fatal("Error getting Device.Interface property: ", err)
	}
	if routeIface != deviceIface {
		s.Fatalf("Wrong route interface: got %s, want Device interface: %s", routeIface, deviceIface)
	}

	// This pattern also comes from src/third_party/autotest/files/client/cros/network.py
	// and is undocumented.
	const downloadBytes = 65536
	fetchURL := fmt.Sprintf("http://%s/download?size=%d", hostName, downloadBytes)
	s.Log("Fetch URL: ", fetchURL)

	// Get data from |fetchURL| and confirm that the correct number of bytes are received.
	resp, err := http.Get(fetchURL)
	if err != nil {
		s.Fatalf("Error fetching data from URL %q: %s", fetchURL, err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		s.Fatal("Error reading data: ", err)
	}
	bytesRead := len(body)
	if bytesRead != downloadBytes {
		s.Fatalf("Read wrong number of bytes: got %d, want %d", bytesRead, downloadBytes)
	}
}
