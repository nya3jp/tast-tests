// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"github.com/godbus/dbus"

	"chromiumos/tast/common/crypto/certificate"
	"chromiumos/tast/common/shillconst"
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
		Attr:         []string{"group:mainline"},
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
						shillconst.ServicePropertyEAPInnerEAP: "auth=MSCHAPV2",
					},
				},
			},
		},
	})
}

const (
	// Shill profile name
	testProfile = "test"
	// Credentials
	identity = "testuser"
	password = "password"
)

type testContext struct {
	param       *testParameters
	certs       *certificate.CertStore
	manager     *shill.Manager
	device      *shill.Device
	profilePath dbus.ObjectPath
	veth        *veth.Pair
	server      *wiredhostapd.Server
	outdir      string
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
func Ethernet8021X(fullCtx context.Context, s *testing.State) {
	// Reserve a little time for cleanup.
	ctx, cancel := ctxutil.Shorten(fullCtx, 5*time.Second)
	defer cancel()

	tc, err := initializeTest(ctx, s)
	if err != nil {
		s.Fatal("Failed to initialize test: ", err)
	}
	defer func() {
		if err := cleanupTest(fullCtx, tc); err != nil {
			s.Error("Failed to clean up: ", err)
		}
	}()

	// Before the server starts up, Shill shouldn't think we're doing EAP.
	s.Log("Ensure EAP is not detected yet")
	if err := waitForDeviceProperty(ctx, tc.device, shillconst.DevicePropertyEapDetected, false); err != nil {
		s.Fatal("Failed to check initial EAP stat: ", err)
	}

	if err := startServer(ctx, tc); err != nil {
		s.Fatal("Failed to start server: ", err)
	}
	defer func() {
		if err := stopServer(fullCtx, tc); err != nil {
			s.Error("Failed to stop server: ", err)
		}
	}()

	if err := testAuthDetection(ctx, tc); err != nil {
		s.Fatal("Failed to initiate authentication: ", err)
	}

	if err := testAuthentication(ctx, tc); err != nil {
		s.Fatal("Failed to authenticate: ", err)
	}

	if err := testDeauthentication(ctx, tc); err != nil {
		s.Fatal("Failed to deauthenticate: ", err)
	}

	if err := testReauthentication(ctx, tc); err != nil {
		s.Fatal("Failed to reauthenticate: ", err)
	}
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

	props, err := d.GetShillProperties(ctx)
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

func initializeTest(ctx context.Context, s *testing.State) (tc *testContext, err error) {
	const (
		vethIface = "test_ethernet"
		// NB: Shill explicitly avoids managing interfaces whose name is prefixed with 'veth'.
		hostapdIface = "veth_test"
	)

	param := s.Param().(testParameters)
	cert := certificate.TestCert1()

	m, err := shill.NewManager(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed creating shill manager proxy")
	}

	profilePath, err := prepareProfile(ctx, m, testProfile)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare profiles")
	}
	defer func() {
		if err != nil {
			if err2 := cleanupProfile(ctx, m, testProfile); err2 != nil {
				testing.ContextLog(ctx, "Failed to cleanup profile: ", err2)
			}
		}
	}()

	// Prepare virtual ethernet link.
	vEth, err := veth.NewPair(ctx, vethIface, hostapdIface)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create veth pair")
	}
	defer func() {
		if err != nil {
			if err2 := vEth.Delete(ctx); err2 != nil {
				testing.ContextLog(ctx, "Failed to cleanup veth: ", err2)
			}
		}
	}()

	d, err := m.WaitForDeviceByName(ctx, vEth.Iface.Name, time.Second*3)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find veth device managed by Shill")
	}

	return &testContext{
		param:       &param,
		certs:       &cert,
		manager:     m,
		device:      d,
		profilePath: profilePath,
		veth:        vEth,
		outdir:      s.OutDir(),
	}, nil
}

// cleanupTest performs the teardown of initializeTest.
func cleanupTest(ctx context.Context, tc *testContext) error {
	err := tc.veth.Delete(ctx)
	if err != nil {
		err = errors.Wrap(err, "failed to delete veth")
	}
	if err2 := cleanupProfile(ctx, tc.manager, testProfile); err2 != nil {
		if err != nil {
			testing.ContextLog(ctx, "Previous error: ", err)
		}
		return errors.Wrap(err2, "failed to clean up profile")
	}
	return err
}

func startServer(ctx context.Context, tc *testContext) error {
	testing.ContextLog(ctx, "Starting hostapd server")

	// Hostapd performs as the authentication server.
	tc.server = &wiredhostapd.Server{
		Iface: tc.veth.PeerIface.Name,
		EAP: &wiredhostapd.EAPConf{
			OuterAuth: tc.param.outerAuth,
			InnerAuth: tc.param.innerAuth,
			Identity:  identity,
			Password:  password,
			Cert:      tc.certs,
		},
		OutDir: tc.outdir,
	}
	if err := tc.server.Start(ctx); err != nil {
		return errors.Wrap(err, "failed to start hostapd")
	}
	return nil
}

func stopServer(ctx context.Context, tc *testContext) error {
	if err := tc.server.Stop(ctx); err != nil {
		return errors.Wrap(err, "failed to stop hostapd")
	}
	return nil
}

func testAuthDetection(ctx context.Context, tc *testContext) error {
	testing.ContextLog(ctx, "Initiating EAP from hostapd")
	if err := tc.server.StartEAPOL(ctx); err != nil {
		return errors.Wrap(err, "failed to start EAPOL")
	}

	testing.ContextLog(ctx, "Waiting for EAP detection")
	if err := waitForDeviceProperty(ctx, tc.device, shillconst.DevicePropertyEapDetected, true); err != nil {
		return errors.Wrap(err, "failed to detect EAP")
	}

	return nil
}

// waitForAuth waits for the appropriate client and server authentication states. If auth is true, we wait for
// authentication to complete successfully. If auth is false, we wait for authentication logoff.
func waitForAuth(ctx context.Context, tc *testContext, auth bool) error {
	var staStatusProperty string
	if auth {
		staStatusProperty = wiredhostapd.STAStatusAuthSuccess
	} else {
		staStatusProperty = wiredhostapd.STAStatusAuthLogoff
	}

	clientErr := waitForDeviceProperty(ctx, tc.device, shillconst.DevicePropertyEapCompleted, auth)
	// Check in with hostapd in either failure or success.
	serverErr := tc.server.ExpectSTAStatus(ctx, tc.veth.Iface.HardwareAddr.String(), staStatusProperty, "1")
	if clientErr != nil {
		return errors.Wrapf(clientErr, "failed waiting for client auth==%t", auth)
	} else if serverErr != nil {
		return errors.Wrapf(serverErr, "server failed to reach auth==%t", auth)
	}

	return nil
}

func testAuthentication(ctx context.Context, tc *testContext) error {
	// Configure client authentication parameters.
	eap := map[string]interface{}{
		shillconst.ServicePropertyType:         "etherneteap",
		shillconst.ServicePropertyEAPMethod:    tc.param.outerAuth,
		shillconst.ServicePropertyEAPCACertPEM: []string{tc.certs.CACert},
		shillconst.ServicePropertyEAPPassword:  password,
		shillconst.ServicePropertyEAPIdentity:  identity,
	}
	for k, v := range tc.param.serviceProps {
		eap[k] = v
	}
	if _, err := tc.manager.ConfigureServiceForProfile(ctx, tc.profilePath, eap); err != nil {
		return errors.Wrap(err, "failed to configure service")
	}

	testing.ContextLog(ctx, "Waiting for Ethernet EAP auth")
	return waitForAuth(ctx, tc, true)
}

func testDeauthentication(ctx context.Context, tc *testContext) error {
	// Popping the test profile (and EAP credentials) should initiate log-off.
	if err := tc.manager.PopProfile(ctx, testProfile); err != nil {
		return errors.Wrap(err, "failed to pop profile")
	}

	testing.ContextLog(ctx, "Waiting for Ethernet EAP logoff")
	return waitForAuth(ctx, tc, false)
}

func testReauthentication(ctx context.Context, tc *testContext) error {
	// Push the test profile back, and we should authenticate again.
	if _, err := tc.manager.PushProfile(ctx, testProfile); err != nil {
		return errors.Wrap(err, "failed to push profile")
	}

	// NB: the EAP server is not notified that we've re-established our credentials, so we're waiting on
	// the server state machine (RFC 3748 4.3, Retransmission Behavior) to retry auth. This can take a few
	// seconds.
	testing.ContextLog(ctx, "Waiting for Ethernet EAP reauth")
	return waitForAuth(ctx, tc, true)
}
