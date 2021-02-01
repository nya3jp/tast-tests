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
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/network/cellular"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// Note: This test enables Cellular if not already enabled.

func init() {
	testing.AddTest(&testing.Test{
		Func: ShillCellularSmoke,
		Desc: "Verifies that traffic can be sent over the Cellular network",
		Contacts: []string{
			"stevenjb@google.com",
			"chromeos-cellular-team@google.com",
		},
		Attr: []string{"group:cellular"},
	})
}

func ShillCellularSmoke(ctx context.Context, s *testing.State) {
	helper, err := cellular.NewHelper(ctx)
	if err != nil {
		s.Fatal("Failed to create cellular.Helper: ", err)
	}

	if err := helper.Manager.DisableTechnology(ctx, shill.TechnologyEthernet); err != nil {
		s.Fatal("Unable to disable Ethernet: ", err)
	}
	if err := helper.Manager.DisableTechnology(ctx, shill.TechnologyWifi); err != nil {
		s.Fatal("Unable to disable WiFi: ", err)
	}
	ctxForEnable := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 1*time.Second)
	defer cancel()
	defer func(ctx context.Context) {
		if err := helper.Manager.EnableTechnology(ctx, shill.TechnologyEthernet); err != nil {
			s.Fatal("Unable to enable Ethernet: ", err)
		}
		if err := helper.Manager.EnableTechnology(ctx, shill.TechnologyWifi); err != nil {
			s.Fatal("Unable to enable WiFi: ", err)
		}
	}(ctxForEnable)

	if _, err := helper.FindServiceForDevice(ctx); err != nil {
		s.Fatal("Unable to find Cellular Service for Device: ", err)
	}

	const hostName = "testing-chargen.appspot.com"
	addrs, err := net.LookupHost(hostName)
	if err != nil || len(addrs) < 1 {
		s.Fatalf("Unable to lookup address for host: %s: %s", hostName, err)
	}
	s.Log("IP: ", addrs[0])

	route, err := testexec.CommandContext(ctx, "ip", "route", "get", addrs[0]).Output()
	if err != nil {
		s.Fatal("Error running 'ip route': ", err)
	}
	routeIface := strings.Split(string(route), " ")[4]

	props, err := helper.Device.GetShillProperties(ctx)
	if err != nil {
		s.Fatal("Error getting Device properties: ", err)
	}
	deviceIface, err := props.GetString(shillconst.DevicePropertyInterface)
	if err != nil {
		s.Fatal("Error getting Device.Interface property: ", err)
	}
	if routeIface != deviceIface {
		s.Fatalf("Route interface: %s != Device interface: %s", routeIface, deviceIface)
	}

	const downloadBytes = 65536
	fetchURL := fmt.Sprintf("http://%s/download?size=%d", hostName, downloadBytes)
	s.Log("Fetch URL: ", fetchURL)

	resp, err := http.Get(fetchURL)
	if err != nil {
		s.Fatalf("Error fetching data from URL: %s: %s", fetchURL, err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		s.Fatal("Error reading data: ", err)
	}
	bytesRead := len(body)
	if bytesRead != downloadBytes {
		s.Fatalf("Read %d bytes, expected %d: ", bytesRead, downloadBytes)
	}
}
