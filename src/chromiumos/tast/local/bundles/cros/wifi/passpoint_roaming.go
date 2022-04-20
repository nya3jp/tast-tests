// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/wifi/hwsim"
	"chromiumos/tast/local/bundles/cros/wifi/passpoint"
	"chromiumos/tast/local/hostapd"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

// roamingTest describes the parameters of a single test case.
type roamingTest struct {
	// credentials is the of credentials under test.
	credentials *passpoint.Credentials
	// aps is the list of access points to setup for the test.
	aps []passpoint.AccessPoint
}

func init() {
	// Set of tests designed to reproduce Passpoint "roaming" scenario where a
	// device is expected to be able to match successively with different
	// networks using the same set of credentials. It differs from Wi-Fi roaming
	// in that it is not a move from one access point to another depending on
	// link quality (RSSI, SNR, etc) but a move from one network to another
	// using the same set of credentials.  It aims to reproduce the situation
	// where a device with a set of credentials moves from one place to another
	// and need to connect to the Passpoint networks around.
	testing.AddTest(&testing.Test{
		Func: PasspointRoaming,
		Desc: "Passpoint network roaming tests",
		Contacts: []string{
			"damiendejean@chromium.org", // Test author
			"cros-networking@google.com",
		},
		Fixture:      "shillSimulatedWiFi",
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"wifi", "shill-wifi"},
		LacrosStatus: testing.LacrosVariantUnneeded,
		Timeout:      3 * time.Minute,
		Params: []testing.Param{
			{
				Name: "home_to_home",
				Val: roamingTest{
					credentials: &passpoint.Credentials{
						Domain:  passpoint.BlueDomain,
						HomeOIs: []uint64{passpoint.HomeOI},
					},
					aps: []passpoint.AccessPoint{
						{
							SSID:              "passpoint-green",
							Domain:            passpoint.GreenDomain,
							Realms:            []string{passpoint.GreenDomain},
							RoamingConsortium: passpoint.HomeOI,
							Auth:              passpoint.AuthTTLS,
						}, {
							SSID:              "passpoint-blue",
							Domain:            passpoint.BlueDomain,
							Realms:            []string{passpoint.BlueDomain},
							RoamingConsortium: passpoint.HomeOI,
							Auth:              passpoint.AuthTTLS,
						},
					},
				},
			}, {
				Name: "roaming_to_home",
				Val: roamingTest{
					credentials: &passpoint.Credentials{
						Domain:     passpoint.BlueDomain,
						HomeOIs:    []uint64{passpoint.HomeOI},
						RoamingOIs: []uint64{passpoint.RoamingOI1, passpoint.RoamingOI2},
					},
					aps: []passpoint.AccessPoint{
						{
							SSID:              "passpoint-green",
							Domain:            passpoint.GreenDomain,
							Realms:            []string{passpoint.GreenDomain},
							RoamingConsortium: passpoint.RoamingOI1,
							Auth:              passpoint.AuthTTLS,
						}, {
							SSID:              "passpoint-blue",
							Domain:            passpoint.BlueDomain,
							Realms:            []string{passpoint.BlueDomain},
							RoamingConsortium: passpoint.HomeOI,
							Auth:              passpoint.AuthTTLS,
						},
					},
				},
			}, {
				Name: "home_to_roaming",
				Val: roamingTest{
					credentials: &passpoint.Credentials{
						Domain:     passpoint.BlueDomain,
						HomeOIs:    []uint64{passpoint.HomeOI},
						RoamingOIs: []uint64{passpoint.RoamingOI1, passpoint.RoamingOI2},
					},
					aps: []passpoint.AccessPoint{
						{
							SSID:              "passpoint-blue",
							Domain:            passpoint.BlueDomain,
							Realms:            []string{passpoint.BlueDomain},
							RoamingConsortium: passpoint.HomeOI,
							Auth:              passpoint.AuthTTLS,
						}, {
							SSID:              "passpoint-red",
							Domain:            passpoint.RedDomain,
							Realms:            []string{passpoint.RedDomain},
							RoamingConsortium: passpoint.RoamingOI2,
							Auth:              passpoint.AuthTTLS,
						},
					},
				},
			}, {
				Name: "roaming_to_roaming",
				Val: roamingTest{
					credentials: &passpoint.Credentials{
						Domain:     passpoint.BlueDomain,
						HomeOIs:    []uint64{passpoint.HomeOI},
						RoamingOIs: []uint64{passpoint.RoamingOI1, passpoint.RoamingOI2},
					},
					aps: []passpoint.AccessPoint{
						{
							SSID:              "passpoint-green",
							Domain:            passpoint.GreenDomain,
							Realms:            []string{passpoint.GreenDomain},
							RoamingConsortium: passpoint.RoamingOI1,
							Auth:              passpoint.AuthTTLS,
						}, {
							SSID:              "passpoint-red",
							Domain:            passpoint.RedDomain,
							Realms:            []string{passpoint.RedDomain},
							RoamingConsortium: passpoint.RoamingOI2,
							Auth:              passpoint.AuthTTLS,
						},
					},
				},
			},
		},
	})
}

// roamingTestContext contains the environment required to run a test case.
type roamingTestContext struct {
	// manager is the proxy to Shill Manager API.
	manager *shill.Manager
	// clientIface is the simulated interface used by Shill.
	clientIface string
	// apIface is the simulated access point interface.
	apIface string
	// aps is the list of access point created for the test.
	aps []*hostapd.Server
	// credentials is the set of Passpoint credentials under test.
	credentials *passpoint.Credentials
}

func PasspointRoaming(ctx context.Context, s *testing.State) {
	// Reserve a little time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 15*time.Second)
	defer cancel()

	tc, err := prepareRoamingTest(ctx, s)
	if err != nil {
		s.Fatal("Failed to initialize test: ", err)
	}

	// Create a profile dedicated to the test.
	profileName := passpoint.RandomProfileName()
	profile, err := tc.manager.CreateFakeUserProfile(ctx, profileName)
	if err != nil {
		s.Fatal("Failed to create test profile: ", err)
	}
	defer func(ctx context.Context) {
		// Remove the test profile.
		err := tc.manager.RemoveFakeUserProfile(ctx, profileName)
		if err != nil {
			s.Fatal("Failed to clean test profile: ", err)
		}
	}(cleanupCtx)

	// Allow Shill to perform interworking select and match networks.
	if err := tc.manager.SetInterworkingSelectEnabled(ctx, tc.clientIface, true); err != nil {
		s.Fatal("Failed to enable interworking selection: ", err)
	}
	defer tc.manager.SetInterworkingSelectEnabled(ctx, tc.clientIface, false)

	// Add the set of credentials to Shill.
	if err := tc.manager.AddPasspointCredentials(ctx, profile, tc.credentials.ToProperties()); err != nil {
		s.Fatal("Failed to set Passpoint credentials: ", err)
	}

	for _, ap := range tc.aps {
		// Create the test access point.
		if err = ap.Start(ctx); err != nil {
			s.Fatal("Failed to start access point: ", err)
		}

		// Trigger a scan
		if err := tc.manager.RequestScan(ctx, shill.TechnologyWifi); err != nil {
			s.Fatal("Failed to request an active scan: ", err)
		}

		// Check the device connects to the access point.
		err = passpoint.WaitForSTAAssociated(ctx, tc.clientIface, ap, time.Minute)
		if err != nil {
			if err := ap.Stop(); err != nil {
				s.Error("Failed to stop access point: ", err)
			}
			s.Fatal("Passpoint client not connected to access point: ", err)
		}

		if err = ap.Stop(); err != nil {
			s.Fatal("Failed to stop access point: ", err)
		}
	}
}

// prepareRoamingTest creates a test profile, reads the test parameters and delivers a test context.
func prepareRoamingTest(ctx context.Context, s *testing.State) (*roamingTestContext, error) {
	m, err := shill.NewManager(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to shill Manager")
	}

	// Obtain the simulated interfaces from the fixture environment.
	ifaces := s.FixtValue().(*hwsim.ShillSimulatedWiFi)
	if len(ifaces.AP) < 1 {
		return nil, errors.Wrap(err, "roaming test require at least one simulated interface")
	}
	if len(ifaces.Client) < 1 {
		return nil, errors.Wrap(err, "roaming test require at least one simulated client interface")
	}

	// Obtain the test case.
	params := s.Param().(roamingTest)

	// Create one access point per test network.
	var servers []*hostapd.Server
	for _, ap := range params.aps {
		server := ap.ToServer(ifaces.AP[0], s.OutDir())
		servers = append(servers, server)
	}

	return &roamingTestContext{
		manager:     m,
		clientIface: ifaces.Client[0],
		apIface:     ifaces.AP[0],
		credentials: params.credentials,
		aps:         servers,
	}, nil
}
