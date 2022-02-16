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

// FixtureIfaces contains the Wi-Fi interfaces created by the simulated
// environment.
type FixtureIfaces struct {
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
		Name: "shillSimulatedWifi",
		Desc: "A fixture that loads the Wi-Fi hardware simulator and ensures Shill is configured correctly",
		Contacts: []string{
			"damiendejean@google.com", // fixture maintainer
		},
		SetUpTimeout:    hwsimTimeout,
		TearDownTimeout: hwsimTimeout,
		ResetTimeout:    hwsimTimeout,
		Impl:            &fixture{},
	})
}

func (f *fixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	// Unload the module if it's already loaded
	loaded, err := isLoaded()
	if err != nil {
		s.Fatal("Failed to check for hwsim module state: ", err)
	}
	if loaded {
		err = unload(ctx)
		if err != nil {
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
			if err != nil {
				if err2 := f.m.ReleaseInterface(ctx, testIfaceClaimer, f.hwIface); err2 != nil {
					s.Errorf("Failed to release interface %s: %v", f.hwIface, err2)
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
		if err != nil {
			if err2 := unload(ctx); err2 != nil {
				s.Error("Failed to unload simulation driver: ", err2)
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

	return &FixtureIfaces{
		Client: clientIface,
		AP:     apIfaces,
	}
}

func (f *fixture) TearDown(ctx context.Context, s *testing.FixtState) {
	// Unload the simulation driver.
	err := unload(ctx)
	if err != nil {
		s.Error("Failed to unload simulation driver: ", err)
	}

	// Give the hardware interface back to Shill if any.
	if f.hwIface != "" {
		err = f.m.ReleaseInterface(ctx, testIfaceClaimer, f.hwIface)
		if err != nil {
			s.Fatalf("Failed to release hardware interface %s: %v", f.hwIface, err)
		}
	}
}

func (f *fixture) Reset(ctx context.Context) error {
	_, _, pid, err := upstart.JobStatus(ctx, "shill")
	if err != nil {
		return errors.Wrap(err, "failed to obtain Shill PID")
	}
	if f.pid != pid {
		return errors.New("failed to maintain fixture start: Shill restarted")
	}

	return nil
}

func (f *fixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
}

func (f *fixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
}
