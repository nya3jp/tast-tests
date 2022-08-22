// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/wifi/hwsim"
	"chromiumos/tast/local/bundles/cros/wifi/passpoint"
	"chromiumos/tast/local/hostapd"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

func init() {
	// Set of tests designed to reproduce Passpoint provisioning interaction from
	// ARC. It is expected for the device to connect successfully to networks
	// with different Passpoint credentials. Once the credentials is removed from
	// ARC, the device is expected to disconnect from the network.
	testing.AddTest(&testing.Test{
		Func: PasspointARCProvisioning,
		Desc: "Passpoint network ARC provisioning tests",
		Contacts: []string{
			"jasongustaman@google.com",
			"damiendejean@google.com",
			"cros-networking@google.com",
		},
		Fixture:      "shillSimulatedWiFiWithArcBooted",
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"wifi", "shill-wifi", "chrome", "arc"},
		LacrosStatus: testing.LacrosVariantUnneeded,
		Timeout:      7 * time.Minute,
	})
}

// passpointARCProvisioningTestCase is a structure to hold the test's cases.
type passpointARCProvisioningTestCase struct {
	desc  string
	ap    passpoint.AccessPoint
	creds passpoint.Credentials
}

func PasspointARCProvisioning(ctx context.Context, s *testing.State) {
	// Fully qualified domain name used to connect to the AP.
	// This value must match the domain of the certificate used by the AP, chromiumos/tast/common/crypto/certificate TestCert1().
	// This is because ARC fills its EAP domain suffix match with Passpoint credentials' FQDN.
	const fqdn = "chromelab-wifi-testbed-server.mtv.google.com"

	var tcs = []passpointARCProvisioningTestCase{
		{
			desc: "TTLS with Home OI",
			ap: passpoint.AccessPoint{
				SSID:               "passpoint-ttls-home-oi",
				Domain:             fqdn,
				Realms:             []string{fqdn},
				RoamingConsortiums: []uint64{passpoint.HomeOI},
				Auth:               passpoint.AuthTTLS,
			},
			creds: passpoint.Credentials{
				Domains: []string{fqdn},
				HomeOIs: []uint64{passpoint.HomeOI},
				Auth:    passpoint.AuthTTLS,
			},
		},
		{
			desc: "TTLS with Roaming OI",
			ap: passpoint.AccessPoint{
				SSID:               "passpoint-ttls-roaming-oi",
				Domain:             fqdn,
				Realms:             []string{fqdn},
				RoamingConsortiums: []uint64{passpoint.RoamingOI1},
				Auth:               passpoint.AuthTTLS,
			},
			creds: passpoint.Credentials{
				Domains:    []string{fqdn},
				HomeOIs:    []uint64{passpoint.HomeOI},
				RoamingOIs: []uint64{passpoint.RoamingOI1},
				Auth:       passpoint.AuthTTLS,
			},
		},
		{
			desc: "TLS with Home OI",
			ap: passpoint.AccessPoint{
				SSID:               "passpoint-tls-home-oi",
				Domain:             fqdn,
				Realms:             []string{fqdn},
				RoamingConsortiums: []uint64{passpoint.HomeOI},
				Auth:               passpoint.AuthTLS,
			},
			creds: passpoint.Credentials{
				Domains: []string{fqdn},
				HomeOIs: []uint64{passpoint.HomeOI},
				Auth:    passpoint.AuthTLS,
			},
		},
		{
			desc: "TLS with Roaming OI",
			ap: passpoint.AccessPoint{
				SSID:               "passpoint-tls-roaming-oi",
				Domain:             fqdn,
				Realms:             []string{fqdn},
				RoamingConsortiums: []uint64{passpoint.RoamingOI1},
				Auth:               passpoint.AuthTLS,
			},
			creds: passpoint.Credentials{
				Domains:    []string{fqdn},
				HomeOIs:    []uint64{passpoint.HomeOI},
				RoamingOIs: []uint64{passpoint.RoamingOI1},
				Auth:       passpoint.AuthTLS,
			},
		},
	}

	m, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed to connect to shill Manager: ", err)
	}

	// Obtain the simulated interfaces from the fixture environment.
	ifaces := s.FixtValue().(*hwsim.ShillSimulatedWiFi)
	if len(ifaces.AP) < 1 {
		s.Fatal("Test requires at least one simulated interface")
	}
	if len(ifaces.Client) < 1 {
		s.Fatal("Test requires at least one simulated client interface")
	}

	// Allow Shill to perform interworking select and match networks.
	if err := m.SetInterworkingSelectEnabled(ctx, ifaces.Client[0], true); err != nil {
		s.Fatal("Failed to enable interworking selection: ", err)
	}
	defer m.SetInterworkingSelectEnabled(ctx, ifaces.Client[0], false)

	// Get ARC handle to provision credentials.
	a := s.FixtValue().(*hwsim.ShillSimulatedWiFi).ARC
	for _, tc := range tcs {
		if err := runARCProvisioningTestCase(ctx, s, m, a, ifaces.AP[0], ifaces.Client[0], tc); err != nil {
			s.Errorf("Failed to complete provisioning test with %s: %v", tc.desc, err)
		}
	}
}

// runARCProvisioningTestCase expects an association after a successful ARC provision followed by a dissociation after ARC removed the credentials.
func runARCProvisioningTestCase(ctx context.Context, s *testing.State, m *shill.Manager, a *arc.ARC, apIface, clientIface string, tc passpointARCProvisioningTestCase) (retErr error) {
	server := tc.ap.ToServer(apIface, s.OutDir())
	if err := server.Start(ctx); err != nil {
		return errors.Wrap(err, "failed to start access point")
	}
	defer server.Stop()

	// Reserve a little time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(cleanupCtx, 5*time.Second)
	defer cancel()

	// Create a monitor to collect access point events.
	h := hostapd.NewMonitor()
	if err := h.Start(ctx, server); err != nil {
		return errors.Wrap(err, "failed to start hostapd monitor")
	}
	defer func(ctx context.Context) {
		if err := h.Stop(ctx); retErr == nil && err != nil {
			retErr = errors.Wrap(err, "failed to stop hostapd monitor")
		}
	}(cleanupCtx)

	// Provision Passpoint credentials from ARC.
	config, err := tc.creds.ToAndroidConfig(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create Android's config")
	}
	if err := a.Command(ctx, "cmd", "wifi", "add-passpoint-config", config).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to add Passpoint config from ARC")
	}
	removed := false
	defer func(ctx context.Context) {
		if removed {
			return
		}
		a.Command(ctx, "cmd", "wifi", "remove-passpoint-config", tc.creds.FQDN()).Run(testexec.DumpLogOnError)
	}(cleanupCtx)

	// Trigger a scan.
	if err := m.RequestScan(ctx, shill.TechnologyWifi); err != nil {
		return errors.Wrap(err, "failed to request an active scan")
	}

	// Wait for the station to associate with the access point.
	if err := passpoint.WaitForSTAAssociated(ctx, h, clientIface, passpoint.STAAssociationTimeout); err != nil {
		return errors.Wrap(err, "failed to check station association")
	}

	// Remove Passpoint credentials from ARC.
	removed = true
	if err := a.Command(ctx, "cmd", "wifi", "remove-passpoint-config", tc.creds.FQDN()).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to remove Passpoint config from ARC")
	}

	// Wait for the station to dissociate with the access point.
	if err := passpoint.WaitForSTADissociated(ctx, h, clientIface, passpoint.STAAssociationTimeout); err != nil {
		return errors.Wrap(err, "failed to check station dissociation")
	}

	return nil
}
