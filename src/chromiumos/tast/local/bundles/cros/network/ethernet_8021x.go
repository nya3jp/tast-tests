// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"net"
	"time"

	"github.com/godbus/dbus"

	"chromiumos/tast/common/crypto/certificate"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/network/veth"
	"chromiumos/tast/local/bundles/cros/network/wiredhostapd"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

type testParameters struct {
	outerAuth    string
	innerAuth    string
	serviceProps map[string]interface{}
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Ethernet8021X,
		Desc:         "Verifies we can authenticate Ethernet via 802.1X",
		Contacts:     []string{"briannorris@chromium.org", "cros-networking@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"wired_8021x"},

		Params: []testing.Param{
			{
				Name: "peap_mschapv2",
				Val: testParameters{
					outerAuth: "PEAP",
					innerAuth: "MSCHAPV2",
				},
			},
			{
				Name: "ttls_mschapv2",
				Val: testParameters{
					outerAuth: "TTLS",
					innerAuth: "TTLS-MSCHAPV2",
					serviceProps: map[string]interface{}{
						shill.ServicePropertyEAPInnerEAP: "auth=MSCHAPV2",
					},
				},
			},
		},
	})
}

// prepareProfile gets the initial profile state in shape (e.g., clear out leftover profiles, EAP configs,
// etc.; setup test profile).
func prepareProfile(ctx context.Context, m *shill.Manager, profileName string) (dbus.ObjectPath, error) {
	const eapProfileEntry = "etherneteap_all"

	// Clear out all existing Ethernet EAP properties, to avoid pollination of service Properties from one
	// test to the next.
	profiles, err := m.Profiles(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to get profiles")
	}
	for _, p := range profiles {
		// Ignore errors, since a clean profile may not have this entry.
		p.DeleteEntry(ctx, eapProfileEntry)
	}

	// Clear test profile, in case it exists, but ignore errors.
	m.PopProfile(ctx, profileName)
	m.RemoveProfile(ctx, profileName)
	if _, err := m.CreateProfile(ctx, profileName); err != nil {
		return "", errors.Wrap(err, "failed to create profile")
	}
	path, err := m.PushProfile(ctx, profileName)
	if err != nil {
		return "", errors.Wrap(err, "failed to push profile")
	}

	return path, nil
}

func cleanupProfile(ctx context.Context, m *shill.Manager, profileName string) error {
	if err := m.PopProfile(ctx, profileName); err != nil {
		return errors.Wrapf(err, "failed to pop profile %q", profileName)
	}
	if err := m.RemoveProfile(ctx, profileName); err != nil {
		return errors.Wrapf(err, "failed to remove profile %q", profileName)
	}
	return nil
}

func waitForDeviceProperty(ctx context.Context, d *shill.Device, prop string, expected bool) error {
	pw, err := d.CreateWatcher(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create a PropertiesWatcher")
	}
	defer pw.Close(ctx)

	props, err := d.GetProperties(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get device properties")
	}

	if v, err := props.GetBool(prop); err != nil {
		return errors.Wrapf(err, "missing Device property %q", prop)
	} else if v == expected {
		return nil
	}

	// Keep a limited timeout for the property-watch.
	tctx, cancel := context.WithTimeout(ctx, time.Second*15)
	defer cancel()
	for {
		v, err := pw.WaitAll(tctx, prop)
		if err != nil {
			return errors.Wrapf(err, "failed waiting for Device property %q==%t", prop, expected)
		}
		if b := v[0].(bool); b == expected {
			return nil
		}
	}
}

// Ethernet8021X tests 802.1X over Ethernet, using a virtual Ethernet pair. For the authentication server, use
// the EAP server built into hostapd.
//
// Test outline:
//  * Start up hostapd on one end of the link
//  * Ensure client link (managed by Shill) transitions to "EAP detected"
//  * Configure client EAP parameters in Shill
//  * Ensure client transitions to "EAP connected", hostapd transitions to "authorized"
//  * Perform client logout
//  * Ensure client transitions to "EAP not connected", hostapd transitions to "logoff"
//  * Re-login client
//  * Ensure client transitions back to "EAP connected", hostapd transitions to "authorized"
//
// Note that we only test that authentication completes successfully; we don't, e.g., start up a DHCP server,
// so the client never actually gets an IP address. Real 802.1X-enabled switches would typically have
// additional infrastructure to bridge the client over to the "main" network after authentication; hostapd
// doesn't implement this feature, and we consider that beyond the scope of an "authentication" test.
func Ethernet8021X(ctx context.Context, s *testing.State) {
	const (
		testProfile = "test"
		vethIface   = "test_ethernet"
		// NB: Shill explicitly avoids managing interfaces whose name is prefixed with 'veth'.
		hostapdIface = "veth_test"
		identity     = "testuser"
		password     = "password"
	)

	param := s.Param().(testParameters)
	cert := certificate.GetTestCertificate()

	// Reserve a little time for cleanup.
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	m, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed creating shill manager proxy: ", err)
	}

	profilePath, err := prepareProfile(ctx, m, testProfile)
	if err != nil {
		s.Fatal("Failed to prepare profiles: ", err)
	}
	defer func() {
		if err := cleanupProfile(ctx, m, testProfile); err != nil {
			s.Error("Failed to cleanup profile: ", err)
		}
	}()

	// Prepare virtual ethernet link.
	vEth, err := veth.NewPair(ctx, vethIface, hostapdIface)
	if err != nil {
		s.Fatal("Failed to create veth pair: ", err)
	}
	defer func() {
		if err := vEth.Delete(ctx); err != nil {
			s.Error("Failed to cleanup veth: ", err)
		}
	}()

	var vethMAC string
	if i, err := net.InterfaceByName(vEth.Iface); err != nil {
		s.Fatal("Failed to get interface info: ", err)
	} else {
		vethMAC = i.HardwareAddr.String()
	}

	s.Log("Waiting for Ethernet Device")
	d, err := m.WaitForDeviceByName(ctx, vEth.Iface, time.Second*3)
	if err != nil {
		s.Fatal("Failed to find veth device managed by Shill: ", err)
	}

	s.Log("Ensure EAP is not detected yet")
	if err := waitForDeviceProperty(ctx, d, shill.DevicePropertyEapDetected, false); err != nil {
		s.Fatal("Failed to check initial EAP stat: ", err)
	}

	s.Log("Starting hostapd server")
	// Hostapd performs as the authentication server.
	server := wiredhostapd.Server{
		Iface: vEth.PeerIface,
		EAP: &wiredhostapd.EAPConf{
			OuterAuth: param.outerAuth,
			InnerAuth: param.innerAuth,
			Identity:  identity,
			Password:  password,
			Cert:      cert,
		},
		OutDir: s.OutDir(),
	}
	if err := server.Start(ctx); err != nil {
		s.Fatal("Failed to start hostapd: ", err)
	}
	defer func() {
		if err := server.Stop(ctx); err != nil {
			s.Error("Failed to stop hostapd: ", err)
		}
	}()

	s.Log("Initiating EAP from hostapd")
	if err := server.StartEAPOL(ctx); err != nil {
		s.Fatal("Failed to start EAPOL: ", err)
	}

	s.Log("Waiting for EAP detection")
	if err := waitForDeviceProperty(ctx, d, shill.DevicePropertyEapDetected, true); err != nil {
		s.Fatal("Failed to detect EAP: ", err)
	}

	// Configure client authentication parameters.
	eap := map[string]interface{}{
		shill.ServicePropertyType:         "etherneteap",
		shill.ServicePropertyEAPMethod:    param.outerAuth,
		shill.ServicePropertyEAPCACertPEM: []string{cert.ClientCert},
		shill.ServicePropertyEAPPassword:  password,
		shill.ServicePropertyEAPIdentity:  identity,
	}
	for k, v := range param.serviceProps {
		eap[k] = v
	}
	if _, err := m.ConfigureServiceForProfile(ctx, profilePath, eap); err != nil {
		s.Fatal("Failed to configure service: ", err)
	}

	s.Log("Waiting for Ethernet EAP auth")
	clientErr := waitForDeviceProperty(ctx, d, shill.DevicePropertyEapCompleted, true)
	// Check in with hostapd in either failure or success.
	serverErr := server.ExpectSTAStatus(ctx, vethMAC, wiredhostapd.STAStatusAuthSucess, "1")
	if clientErr != nil {
		s.Fatal("Failed to complete EAP: ", clientErr)
	} else if serverErr != nil {
		s.Fatal("Server failed to authorize: ", serverErr)
	}

	// Popping the test profile (and EAP credentials) should initiate log-off.
	if err := m.PopProfile(ctx, testProfile); err != nil {
		s.Fatal("Failed to pop profile: ", err)
	}

	s.Log("Waiting for Ethernet EAP logoff")
	clientErr = waitForDeviceProperty(ctx, d, shill.DevicePropertyEapCompleted, false)
	// Check in with hostapd in either failure or success.
	serverErr = server.ExpectSTAStatus(ctx, vethMAC, wiredhostapd.STAStatusAuthLogoff, "1")
	if clientErr != nil {
		s.Fatal("Failed to deauth EAP: ", clientErr)
	} else if serverErr != nil {
		s.Fatal("Server failed to logoff: ", serverErr)
	}

	// Push the test profile back, and we should authenticate again.
	if _, clientErr := m.PushProfile(ctx, testProfile); clientErr != nil {
		s.Fatal("Failed to push profile: ", clientErr)
	}

	// NB: the EAP server is not notified that we've re-established our credentials, so we're waiting on
	// the server state machine (RFC 3748 4.3, Retransmission Behavior) to retry auth. This can take a few
	// seconds.
	s.Log("Waiting for Ethernet EAP reauth")
	clientErr = waitForDeviceProperty(ctx, d, shill.DevicePropertyEapCompleted, true)
	// Check in with hostapd in either failure or success.
	serverErr = server.ExpectSTAStatus(ctx, vethMAC, wiredhostapd.STAStatusAuthSucess, "1")
	if clientErr != nil {
		s.Fatal("Failed to reauth: ", clientErr)
	} else if serverErr != nil {
		s.Fatal("Server failed to reauth: ", serverErr)
	}
}
