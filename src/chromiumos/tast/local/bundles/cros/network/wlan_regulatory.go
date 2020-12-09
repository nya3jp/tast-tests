// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/network/regdb"
	network_iface "chromiumos/tast/local/network/iface"
	"chromiumos/tast/local/network/iw"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: WlanRegulatory,
		// Test notes: We don't verify that the system truly respects the regulatory database rules, but only that it does not
		// reject them. Note that some WiFi drivers "self manage" their domain detection and so this test can't apply everywhere.
		Desc:     "Ensure the regulatory database is sane and that we can switch domains using the 'iw' utility",
		Contacts: []string{"briannorris@chromium.org", "chromeos-kernel-wifi@google.com"},
		// This test doesn't technically require the wificell fixture, but it's best if non-default regulatory settings are used
		// only in RF chambers.
		Attr:         []string{"group:wificell", "wificell_func", "wificell_unstable"},
		SoftwareDeps: []string{"wifi", "shill-wifi"},
		HardwareDeps: hwdep.D(
			// Marvell systems have their country hard-coded in EEPROM.
			hwdep.SkipOnModel("bob"),
			hwdep.SkipOnModel("elm"),
			hwdep.SkipOnModel("fievel"),
			hwdep.SkipOnModel("hana"),
			hwdep.SkipOnModel("kevin"),
			hwdep.SkipOnModel("tiger"),
		),
	})
}

func WlanRegulatory(ctx context.Context, s *testing.State) {
	manager, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed creating shill manager proxy: ", err)
	}

	iface, err := shill.WifiInterface(ctx, manager, 5*time.Second)
	if err != nil {
		s.Fatal("Could not get a WiFi interface: ", err)
	}
	s.Log("WiFi interface: ", iface)
	phy, err := network_iface.NewInterface(iface).PhyName(ctx)
	if err != nil {
		s.Fatal("Failed to get phy name: ", err)
	}

	iwr := iw.NewLocalRunner()
	if selfManaged, err := iwr.IsRegulatorySelfManaged(ctx); err != nil {
		s.Fatal("Failed to retrieve regulatory status: ", err)
	} else if selfManaged {
		s.Fatal("Test does not work on self-managed wiphys")
	}

	initialDomain, err := iwr.RegulatoryDomain(ctx)
	if err != nil {
		s.Fatal("Failed to retrieve domain: ", err)
	}
	defer func(ctx context.Context) {
		err := iwr.SetRegulatoryDomain(ctx, initialDomain)
		if err != nil {
			s.Error("Failed to reset domain: ", err)
		}
	}(ctx)
	ctx, cancel := ctxutil.Shorten(ctx, time.Second)
	defer cancel()

	db, err := regdb.NewRegulatoryDB()
	if err != nil {
		s.Fatal("Failed to retrieve regulatory database: ", err)
	}

	for i, c := range db.Countries {
		s.Logf("Country %d = %s", i, c.Alpha)
		err := iwr.SetRegulatoryDomain(ctx, c.Alpha)
		if err != nil {
			s.Fatalf("Failed to set country code %s: %v", c.Alpha, err)
		}

		// The kernel processes changes asynchronously, so poll for a short time.
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			// Ask for the phy-specific domain, to ensure that (if it's not fully "self-managed") it still respects the global
			// configuration.
			dom, err := iwr.PhyRegulatoryDomain(ctx, phy)
			if err != nil {
				return testing.PollBreak(errors.Wrap(err, "failed to get wiphy domain"))
			}
			if dom != c.Alpha {
				return errors.Errorf("unexpected country: %q != %q", dom, c.Alpha)
			}
			return nil
		}, &testing.PollOptions{Timeout: time.Second}); err != nil {
			s.Error("Failed to change domains: ", err)
		}
	}
}
