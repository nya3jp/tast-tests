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

// roamingTest describes the parameters of a single test case.
type roamingTest struct {
	// Set of credentials to test
	credentials *passpoint.Credentials
	// List of access point to setup for the test
	aps []passpoint.AccessPoint
}

func init() {
	testing.AddTest(&testing.Test{
		Func: PasspointRoaming,
		Desc: "Passpoint network roaming tests",
		Contacts: []string{
			"damiendejean@chromium.org", // Test author
		},
		Fixture:      "shillSimulatedWifi",
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"wifi", "shill-wifi"},
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
	// Shill Manager API
	manager *shill.Manager
	// Test profile path
	profile dbus.ObjectPath
	// Simulated interface used by Shill as a client interface
	clientIface string
	// Simulated access point interface
	apIface string
	// List of hostapd server
	aps []*hostapd.Server
	// Set of Passpoint properties
	credentials *passpoint.Credentials
}

func PasspointRoaming(fullCtx context.Context, s *testing.State) {
	// Reserve a little time for cleanup.
	ctx, cancel := ctxutil.Shorten(fullCtx, 5*time.Second)
	defer cancel()

	tc, err := prepareRoamingTest(ctx, s)
	if err != nil {
		s.Fatal("Failed to initialize test: ", err)
	}
	defer cleanupRoamingTest(fullCtx, tc)

	// Allow Shill to perform interworking select and match networks.
	err = passpoint.SetInterworkingSelectEnabled(ctx, tc.manager, tc.clientIface, true)
	if err != nil {
		s.Fatal("Failed to enable interworking selection: ", err)
	}
	defer passpoint.SetInterworkingSelectEnabled(ctx, tc.manager, tc.clientIface, false)

	// Add the set of credentials to Shill.
	err = tc.manager.AddPasspointCredentials(ctx, tc.profile, tc.credentials.ToProperties())
	if err != nil {
		s.Fatal("Failed to set Passpoint credentials: ", err)
	}

	for _, ap := range tc.aps {
		// Create the test access point
		err = ap.Start(ctx)
		if err != nil {
			s.Fatal("Failed to start access point: ", err)
		}

		// Check the device connects to the access point.
		err = passpoint.WaitForSTAAssociated(ctx, tc.clientIface, ap, time.Minute)
		if err != nil {
			s.Fatal("Passpoint client not connected to access point: ", err)
		}

		err = ap.Stop(ctx)
		if err != nil {
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

	// Create a profile dedicated to the test
	profilePath, err := passpoint.CreateFakeUserProfile(ctx, m, passpoint.ProfileUser)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare test profile")
	}

	// Obtain the simulated interfaces from the fixture environment.
	ifaces := s.FixtValue().(*hwsim.FixtureIfaces)
	if len(ifaces.AP) < 1 {
		return nil, errors.Wrap(err, "roaming test require at least one simulated interface")
	}

	// Obtain the test case
	params := s.Param().(roamingTest)

	// Create one access point per test network
	var servers []*hostapd.Server
	for _, ap := range params.aps {
		server := ap.ToServer(ifaces.AP[0], s.OutDir())
		servers = append(servers, server)
	}

	return &roamingTestContext{
		manager:     m,
		profile:     profilePath,
		clientIface: ifaces.Client,
		apIface:     ifaces.AP[0],
		credentials: params.credentials,
		aps:         servers,
	}, nil
}

func cleanupRoamingTest(ctx context.Context, tc *roamingTestContext) {
	// Remove the test profile
	err := passpoint.RemoveFakeUserProfile(ctx, tc.manager, passpoint.ProfileUser)
	if err != nil {
		testing.ContextLog(ctx, "Failed to clean test profile: ", err)
	}
}
