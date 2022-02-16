// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package hwsim setups a simulated Wi-Fi environment for testing.
package hwsim

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

const (
	hwsimTimeout        = 30 * time.Second
	shillRequestTimeout = 5 * time.Second
	ifaceCount          = 3
	testIfaceClaimer    = "hwsim-fixture"
)

// ShillSimulatedWiFi contains the Wi-Fi interfaces created by the simulated
// environment.
type ShillSimulatedWiFi struct {
	// Simulated Wi-Fi interface used by Shill a the client interface.
	Client string
	// Simulated Wi-Fi interfaces available to be used as access point
	// interfaces.
	AP []string
}

type fixture struct {
	// Shill Manager interface
	m *shill.Manager
	// Shill PID used to ensure it does not restart while the fixture is running
	pid int
	// Wi-Fi interface already present on the device when the fixture is setup
	hwIface string
}

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: "shillSimulatedWiFi",
		Desc: "A fixture that loads the Wi-Fi hardware simulator and ensures Shill is configured correctly",
		Contacts: []string{
			"damiendejean@google.com", // fixture maintainer
			"cros-networking@google.com",
		},
		SetUpTimeout:    hwsimTimeout,
		TearDownTimeout: hwsimTimeout,
		ResetTimeout:    hwsimTimeout,
		Impl:            &fixture{},
	})
}

func (f *fixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	success := false

	// Unload the module if it's already loaded
	if loaded, err := isLoaded(); err != nil {
		s.Fatal("Failed to check for hwsim module state: ", err)
	} else if loaded {
		if err = unload(ctx); err != nil {
			s.Fatal("Failed to unload hwsim module: ", err)
		}
	}

	// Obtain Shill PID and keep it to ensure to later check the process does
	// not restart.
	_, _, pid, err := upstart.JobStatus(ctx, "shill")
	if err != nil {
		s.Fatal("Failed to obtain Shill PID: ", err)
	}
	f.pid = pid

	f.m, err = shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed to connect to Shill Manager: ", err)
	}

	// Ensure the hardware interface is not in use
	f.hwIface, err = shill.WifiInterface(ctx, f.m, shillRequestTimeout)
	if err == nil {
		// There's a hardware interface, we must tell Shill not to use it.
		err = f.m.ClaimInterface(ctx, testIfaceClaimer, f.hwIface)
		if err != nil {
			s.Fatalf("Failed to claim interface %s: %v", f.hwIface, err)
		}
		defer func() {
			if !success {
				if err := f.m.ReleaseInterface(ctx, testIfaceClaimer, f.hwIface); err != nil {
					s.Fatalf("Failed to release interface %s: %v", f.hwIface, err)
				}
			}
		}()
	}

	// Load the simulation driver (mac80211_hwsim)
	ifaces, err := load(ctx, ifaceCount)
	if err != nil {
		s.Fatal("Failed to load Wi-Fi simulation driver: ", err)
	}
	defer func() {
		if !success {
			if err := unload(ctx); err != nil {
				s.Fatal("Failed to unload simulation driver: ", err)
			}
		}
	}()

	// Retrieve the simulated interface Shill will use.
	clientIface, err := shill.WifiInterface(ctx, f.m, shillRequestTimeout)
	if err != nil {
		s.Fatal("Failed to get Shill simulated interface: ", err)
	}

	// List access point interfaces
	var apIfaces []string
	for _, iface := range ifaces {
		if iface.Name == clientIface {
			continue
		}
		apIfaces = append(apIfaces, iface.Name)
	}

	success = true
	return &ShillSimulatedWiFi{
		Client: clientIface,
		AP:     apIfaces,
	}
}

func (f *fixture) TearDown(ctx context.Context, s *testing.FixtState) {
	// Unload the simulation driver.
	if err := unload(ctx); err != nil {
		s.Error("Failed to unload simulation driver: ", err)
	}

	// Give the hardware interface back to Shill if any.
	if f.hwIface != "" {
		if err := f.m.ReleaseInterface(ctx, testIfaceClaimer, f.hwIface); err != nil {
			s.Fatalf("Failed to release hardware interface %s: %v", f.hwIface, err)
		}
	}
}

func (f *fixture) Reset(ctx context.Context) error {
	if _, _, pid, err := upstart.JobStatus(ctx, "shill"); err != nil {
		return errors.Wrap(err, "failed to obtain Shill PID")
	} else if f.pid != pid {
		return errors.New("failed to maintain fixture state: Shill restarted")
	}

	return nil
}

func (f *fixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
}

func (f *fixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
}
