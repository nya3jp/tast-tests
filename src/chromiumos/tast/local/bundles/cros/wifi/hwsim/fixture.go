// Copyright 2022 The ChromiumOS Authors.
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
	shillIfaceTimeout   = 10 * time.Second
	ifaceCount          = 3
	testIfaceClaimer    = "hwsim-fixture"
)

// ShillSimulatedWiFi contains the Wi-Fi interfaces created by the simulated
// environment.
type ShillSimulatedWiFi struct {
	// Simulated Wi-Fi interfaces used by Shill as client interfaces.
	Client []string
	// Simulated Wi-Fi interfaces available to be used as access point
	// interfaces.
	AP []string
}

type fixture struct {
	// m is the Shill Manager interface.
	m *shill.Manager
	// pid is the Shill process identifier used to ensure it does not restart
	// while the fixture is running.
	pid int
	// hwIface is the name of the Wi-Fi interface already present on the device
	// when the fixture is setup.
	hwIface string
	// claimedIfaces is the a set of interfaces claimed by the fixture to
	// release before unloading the driver.
	claimedIfaces []string
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

	// Ensure the hardware interface is not in use. The call below will return
	// an error if there's multiple Wi-Fi interfaces but it's not something we
	// support at the moment.
	f.hwIface, err = shill.WifiInterface(ctx, f.m, shillRequestTimeout)
	if err == nil {
		// There's a hardware interface, we must tell Shill not to use it.
		err = f.m.ClaimInterface(ctx, testIfaceClaimer, f.hwIface)
		if err != nil {
			s.Fatalf("Failed to claim interface %s: %v", f.hwIface, err)
		}
		defer func(ctx context.Context) {
			if !success {
				if err := f.m.ReleaseInterface(ctx, testIfaceClaimer, f.hwIface); err != nil {
					s.Fatalf("Failed to release interface %s: %v", f.hwIface, err)
				}
			}
		}(ctx)
	}

	// Load the simulation driver (mac80211_hwsim)
	ifaces, err := load(ctx, ifaceCount)
	if err != nil {
		s.Fatal("Failed to load Wi-Fi simulation driver: ", err)
	}
	defer func(ctx context.Context) {
		if !success {
			if err := unload(ctx); err != nil {
				s.Fatal("Failed to unload simulation driver: ", err)
			}
		}
	}(ctx)

	// Wait for all the new interfaces to be managed by Shill.
	// TODO(b/235259730): remove the timeout and find a way to know the number
	// of interfaces expected to be managed by Shill.
	if err := testing.Sleep(ctx, 3*time.Second); err != nil {
		s.Fatal("Failed to wait for Shill to manage interfaces")
	}

	// Obtain the list of Wi-Fi interfaces managed by Shill.
	wm, err := shill.NewWifiManager(ctx, f.m)
	if err != nil {
		s.Fatal("Failed to create Wi-Fi manager: ", err)
	}
	shillIfaces, err := wm.Interfaces(ctx)
	if err != nil {
		s.Fatal("Failed to obtain Wi-Fi interfaces from Shill: ", err)
	}
	if len(ifaces) == 0 {
		s.Fatal("Shill has no Wi-Fi interfaces")
	}

	// Keep track of managed interfaces
	managedIfaces := make(map[string]bool)
	for _, iface := range shillIfaces {
		managedIfaces[iface] = true
	}

	// Keep an interface as client.
	clientIface := shillIfaces[0]

	// Use the other interfaces as test access points.
	var apIfaces []string
	var claimedIfaces []string
	for _, iface := range ifaces {
		if iface.Name == clientIface {
			// The client interface cannot be used as access point and will
			// continue to be managed by Shill.
			continue
		}
		if managedIfaces[iface.Name] {
			// The interface is managed by Shill, we need to claim it before
			// it can be used as a test access point.
			if err := f.m.ClaimInterface(ctx, testIfaceClaimer, iface.Name); err != nil {
				s.Fatalf("Failed to claim interfaces %s: %v", iface.Name, err)
			}
			defer func(ctx context.Context, name string) {
				if !success {
					if err := f.m.ReleaseInterface(ctx, testIfaceClaimer, name); err != nil {
						s.Fatalf("Failed to release interface %s: %v", name, err)
					}
				}
			}(ctx, iface.Name)
			// Keep track of claimed interfaces to release them later.
			claimedIfaces = append(claimedIfaces, iface.Name)
		}
		apIfaces = append(apIfaces, iface.Name)
	}

	f.claimedIfaces = claimedIfaces
	success = true
	return &ShillSimulatedWiFi{
		Client: []string{clientIface},
		AP:     apIfaces,
	}
}

func (f *fixture) TearDown(ctx context.Context, s *testing.FixtState) {
	// Release the simulation driver interfaces claimed in Shill.
	for _, iface := range f.claimedIfaces {
		if err := f.m.ReleaseInterface(ctx, testIfaceClaimer, iface); err != nil {
			s.Errorf("Failed to release interfaces %s: %v", iface, err)
		}
	}

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
