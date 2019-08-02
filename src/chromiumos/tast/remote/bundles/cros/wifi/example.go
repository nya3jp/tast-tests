// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"math/rand"
	"time"

	"chromiumos/tast/dut"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     Example,
		Desc:     "Basic wificell router connection",
		Contacts: []string{"briannorris@chromium.org", "chromeos-kernel-wifi@google.com"},
		// Depends on specialized hardware (wificells). We don't have a
		// way to specify these dependencies yet.
		Attr: []string{"disabled"},
		Vars: []string{"router"},
	})
}

func Example(ctx context.Context, s *testing.State) {
	// TODO: perform DUT hostname mangling by default (e.g.,
	// dut-hostname.cros => dut-host-name-router.cros).
	routerHost := s.RequiredVar("router")

	// TODO: seed this once per run.
	rand.Seed(time.Now().UnixNano())

	d, ok := dut.FromContext(ctx)
	if !ok {
		s.Fatal("Failed to get DUT")
	}

	router, err := wificell.NewRouter(ctx, routerHost)
	if err != nil {
		s.Fatal("Failed to get router: ", err)
	}

	ssid := wificell.RandomSSID("Tast-Y ")
	apConfig := wificell.NewHostAPConfig(ssid)
	ap := wificell.NewHostAPServer(router, apConfig)
	dhcp := wificell.NewDHCPServer(router, "managed0")
	client := wificell.NewWiFiClient(d)

	s.Log("Initializing router")
	if err := router.Initialize(ctx); err != nil {
		s.Fatal("Failed to initialize router: ", err)
	}

	// TODO: match wiphy/wdev with apConfig requirements.
	iface, err := router.GetAPWdev(0)
	if err != nil {
		s.Fatal("Failed to get AP wdev: ", err)
	}

	s.Log("Starting AP")
	if err := ap.Start(ctx, iface); err != nil {
		s.Fatal("Failed to start AP: ", err)
	}
	defer func() {
		if err := ap.Stop(ctx); err != nil {
			s.Fatal("Failed to stop AP: ", err)
		}
	}()

	s.Log("Starting DHCP server")
	if err := dhcp.Start(ctx); err != nil {
		s.Fatal("Failed to start DHCP: ", err)
	}
	defer func() {
		if err := dhcp.Stop(ctx); err != nil {
			s.Fatal("Failed to stop DHCP: ", dhcp)
		}
	}()

	if err := client.Connect(ctx, ssid); err != nil {
		s.Fatal("Failed to connect: ", err)
	}
	defer func() {
		if err := client.Stop(ctx); err != nil {
			s.Fatal("Failed to stop client: ", err)
		}
	}()
}
