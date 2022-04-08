// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"time"

	"github.com/godbus/dbus"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/wifi/hwsim"
	"chromiumos/tast/local/bundles/cros/wifi/passpoint"
	"chromiumos/tast/local/hostapd"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

// selectionTest describes the parameters of a single test case.
type selectionTest struct {
	// Set of credentials used for the test case.
	credentials []*passpoint.Credentials
	// List of test networks
	aps []passpoint.AccessPoint
	// SSID of the network that should be connected at the end of the test
	expectedSSID string
}

func init() {
	testing.AddTest(&testing.Test{
		Func: PasspointSelection,
		Desc: "Passpoint network selection tests",
		Contacts: []string{
			"damiendejean@chromium.org", // Test author
		},
		Fixture:      "shillSimulatedWifi",
		SoftwareDeps: []string{"wifi", "shill-wifi"},
		Params: []testing.Param{
			{
				Name: "home_match_with_domain",
				Val: selectionTest{
					credentials: []*passpoint.Credentials{
						{
							Domain: passpoint.BlueDomain,
						},
					},
					aps: []passpoint.AccessPoint{
						{
							SSID:              "passpoint-blue",
							Domain:            passpoint.BlueDomain,
							Realms:            []string{passpoint.BlueDomain},
							RoamingConsortium: passpoint.HomeOI,
							Auth:              passpoint.AuthTTLS,
						},
					},
					expectedSSID: "passpoint-blue",
				},
			}, {
				Name: "home_match_with_oi",
				Val: selectionTest{
					credentials: []*passpoint.Credentials{
						{
							Domain:  passpoint.BlueDomain,
							HomeOIs: []uint64{passpoint.HomeOI},
						},
					},
					aps: []passpoint.AccessPoint{
						{
							SSID:              "passpoint-green",
							Domain:            passpoint.GreenDomain,
							Realms:            []string{passpoint.GreenDomain},
							RoamingConsortium: passpoint.HomeOI,
							Auth:              passpoint.AuthTTLS,
						},
					},
					expectedSSID: "passpoint-green",
				},
			}, {
				Name: "home_match_with_required_oi",
				Val: selectionTest{
					credentials: []*passpoint.Credentials{
						{
							Domain:          passpoint.BlueDomain,
							RequiredHomeOIs: []uint64{passpoint.HomeOI},
						},
					},
					aps: []passpoint.AccessPoint{
						{
							SSID:              "passpoint-blue",
							Domain:            passpoint.BlueDomain,
							Realms:            []string{passpoint.BlueDomain},
							RoamingConsortium: passpoint.HomeOI,
							Auth:              passpoint.AuthTTLS,
						}, {
							SSID:   "passpoint-another-blue",
							Domain: passpoint.BlueDomain,
							Realms: []string{passpoint.BlueDomain},
							Auth:   passpoint.AuthTTLS,
						},
					},
					expectedSSID: "passpoint-blue",
				},
			}, {
				Name: "roaming_match_with_domain",
				Val: selectionTest{
					credentials: []*passpoint.Credentials{
						{
							Domain: passpoint.BlueDomain,
						},
					},
					aps: []passpoint.AccessPoint{
						{
							SSID:              "passpoint-green",
							Domain:            passpoint.GreenDomain,
							Realms:            []string{passpoint.GreenDomain, passpoint.BlueDomain},
							RoamingConsortium: passpoint.RoamingOI1,
							Auth:              passpoint.AuthTTLS,
						},
					},
					expectedSSID: "passpoint-green",
				},
			}, {
				Name: "roaming_match_with_oi",
				Val: selectionTest{
					credentials: []*passpoint.Credentials{
						{
							Domain:     passpoint.BlueDomain,
							HomeOIs:    []uint64{passpoint.HomeOI},
							RoamingOIs: []uint64{passpoint.RoamingOI1},
						},
					},
					aps: []passpoint.AccessPoint{
						{
							SSID:              "passpoint-green",
							Domain:            passpoint.GreenDomain,
							Realms:            []string{passpoint.GreenDomain},
							RoamingConsortium: passpoint.RoamingOI1,
							Auth:              passpoint.AuthTTLS,
						},
					},
					expectedSSID: "passpoint-green",
				},
			}, {
				Name: "home_over_roaming_ap",
				Val: selectionTest{
					credentials: []*passpoint.Credentials{
						{
							Domain:     passpoint.BlueDomain,
							HomeOIs:    []uint64{passpoint.HomeOI},
							RoamingOIs: []uint64{passpoint.RoamingOI1},
						},
					},
					aps: []passpoint.AccessPoint{
						{
							SSID:              "passpoint-blue",
							Domain:            passpoint.BlueDomain,
							Realms:            []string{passpoint.BlueDomain},
							RoamingConsortium: passpoint.HomeOI,
							Auth:              passpoint.AuthTTLS,
						}, {
							SSID:              "passpoint-green",
							Domain:            passpoint.GreenDomain,
							Realms:            []string{passpoint.GreenDomain},
							RoamingConsortium: passpoint.RoamingOI1,
							Auth:              passpoint.AuthTTLS,
						},
					},
					expectedSSID: "passpoint-blue",
				},
			}, {
				Name: "roaming_match_with_security",
				Val: selectionTest{
					credentials: []*passpoint.Credentials{
						{
							Domain: passpoint.BlueDomain,
						},
					},
					aps: []passpoint.AccessPoint{
						{
							SSID:              "passpoint-ttls",
							Domain:            passpoint.BlueDomain,
							Realms:            []string{passpoint.BlueDomain},
							RoamingConsortium: passpoint.RoamingOI1,
							Auth:              passpoint.AuthTTLS,
						}, {
							SSID:              "passpoint-tls",
							Domain:            passpoint.BlueDomain,
							Realms:            []string{passpoint.BlueDomain},
							RoamingConsortium: passpoint.RoamingOI2,
							Auth:              passpoint.AuthTLS,
						},
					},
					expectedSSID: "passpoint-ttls",
				},
			}, {
				Name: "two_home_credentials",
				Val: selectionTest{
					credentials: []*passpoint.Credentials{
						{
							Domain:  passpoint.BlueDomain,
							HomeOIs: []uint64{passpoint.HomeOI},
						}, {
							Domain:  passpoint.RedDomain,
							HomeOIs: []uint64{passpoint.HomeOI},
						},
					},
					aps: []passpoint.AccessPoint{
						{
							SSID:              "passpoint-blue",
							Domain:            passpoint.BlueDomain,
							Realms:            []string{passpoint.BlueDomain},
							RoamingConsortium: passpoint.HomeOI,
							Auth:              passpoint.AuthTTLS,
						},
					},
					expectedSSID: "passpoint-blue",
				},
			}, {
				Name: "two_roaming_credentials",
				Val: selectionTest{
					credentials: []*passpoint.Credentials{
						{
							Domain:     passpoint.GreenDomain,
							HomeOIs:    []uint64{passpoint.RoamingOI1},
							RoamingOIs: []uint64{passpoint.HomeOI},
						}, {
							Domain:     passpoint.RedDomain,
							HomeOIs:    []uint64{passpoint.RoamingOI2},
							RoamingOIs: []uint64{passpoint.HomeOI},
						},
					},
					aps: []passpoint.AccessPoint{
						{
							SSID:              "passpoint-blue",
							Domain:            passpoint.BlueDomain,
							Realms:            []string{passpoint.BlueDomain},
							RoamingConsortium: passpoint.HomeOI,
							Auth:              passpoint.AuthTTLS,
						},
					},
					expectedSSID: "passpoint-blue",
				},
			}, {
				Name: "home_over_roaming_credentials",
				Val: selectionTest{
					credentials: []*passpoint.Credentials{
						{
							Domain:  passpoint.BlueDomain,
							HomeOIs: []uint64{passpoint.HomeOI},
						}, {
							Domain:     passpoint.RedDomain,
							HomeOIs:    []uint64{passpoint.RoamingOI2},
							RoamingOIs: []uint64{passpoint.RoamingOI1},
						},
					},
					aps: []passpoint.AccessPoint{
						{
							SSID:              "passpoint-blue",
							Domain:            passpoint.BlueDomain,
							Realms:            []string{passpoint.BlueDomain},
							RoamingConsortium: passpoint.HomeOI,
							Auth:              passpoint.AuthTTLS,
						}, {
							SSID:              "passpoint-green",
							Domain:            passpoint.GreenDomain,
							Realms:            []string{passpoint.GreenDomain},
							RoamingConsortium: passpoint.RoamingOI1,
							Auth:              passpoint.AuthTTLS,
						},
					},
					expectedSSID: "passpoint-blue",
				},
			},
		},
	})
}

// selectionTestContext contains the required environment to run the current test case.
type selectionTestContext struct {
	// Shill Manager API
	manager *shill.Manager
	// Test profile path
	profile dbus.ObjectPath
	// List of hostapd server
	aps []*hostapd.Server
	// Set of Passpoint properties
	credentials []*passpoint.Credentials
	// Simulated interface used by Shill as a client interface
	clientIface string
	// Server where to check for device presence
	expectedAP *hostapd.Server
}

func PasspointSelection(fullCtx context.Context, s *testing.State) {
	// Reserve a little time for cleanup.
	ctx, cancel := ctxutil.Shorten(fullCtx, 5*time.Second)
	defer cancel()

	tc, err := prepareSelectionTest(ctx, s)
	if err != nil {
		s.Fatal("Failed to initialize test: ", err)
	}
	defer cleanupSelectionTest(fullCtx, tc)

	// Allow Shill to perform interworking select and match networks.
	err = passpoint.SetInterworkingSelectEnabled(ctx, tc.manager, tc.clientIface, true)
	if err != nil {
		s.Fatal("Failed to enable interworking selection: ", err)
	}
	defer passpoint.SetInterworkingSelectEnabled(ctx, tc.manager, tc.clientIface, false)

	// Start all the access points.
	for _, ap := range tc.aps {
		err = ap.Start(ctx)
		if err != nil {
			s.Fatal("Failed to start access point: ", err)
		}
		defer ap.Stop(ctx)
	}

	// Add the sets of credentials to Shill.
	for _, c := range tc.credentials {
		err = tc.manager.AddPasspointCredentials(ctx, tc.profile, c.ToProperties())
		if err != nil {
			s.Fatal("Failed to set Passpoint credentials: ", err)
		}
	}

	// Check the device connects to the access point.
	err = passpoint.WaitForSTAAssociated(ctx, tc.clientIface, tc.expectedAP, 30*time.Second)
	if err != nil {
		s.Fatal("Passpoint client not connected to access point: ", err)
	}
}

// prepareSelectionTest creates a test profile, reads the test parameters and delivers a test context.
func prepareSelectionTest(ctx context.Context, s *testing.State) (tc *selectionTestContext, err error) {
	m, err := shill.NewManager(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to shill Manager")
	}

	// Create a profile dedicated to the test
	profilePath, err := passpoint.CreateFakeUserProfile(ctx, m, passpoint.ProfileUser)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare test profile")
	}

	// Obtain the simulated interfaces from the fixture environment.
	ifaces := s.FixtValue().(*hwsim.FixtureIfaces)

	// Obtain the parameters for the current test case and ensure it does not
	// require more access point than the fixture provided.
	params := s.Param().(selectionTest)
	if len(params.aps) > len(ifaces.AP) {
		return nil, errors.Errorf("test case requires %d interfaces, fixture setup only %d",
			len(params.aps), len(ifaces.AP))
	}

	// Create one access point per test network
	var servers []*hostapd.Server
	var expectedServer *hostapd.Server
	for i, ap := range params.aps {
		server := ap.ToServer(ifaces.AP[i], s.OutDir())
		servers = append(servers, server)
		if ap.SSID == params.expectedSSID {
			expectedServer = server
		}
	}

	if expectedServer == nil {
		return nil, errors.Errorf("no match between expected SSID %q and created servers", params.expectedSSID)
	}

	return &selectionTestContext{
		manager:     m,
		profile:     profilePath,
		aps:         servers,
		credentials: params.credentials,
		clientIface: ifaces.Client,
		expectedAP:  expectedServer,
	}, nil
}

// cleanupSelectionTest removes the temporary profile to leave the device in a clean state.
func cleanupSelectionTest(ctx context.Context, tc *selectionTestContext) {
	// Remove the test profile
	err := passpoint.RemoveFakeUserProfile(ctx, tc.manager, passpoint.ProfileUser)
	if err != nil {
		testing.ContextLog(ctx, "Failed to clean test profile: ", err)
	}
}
