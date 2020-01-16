// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/common/crypto/certificate"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/local/testexec"
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
		Contacts:     []string{"briannorris@google.com", "cros-networking@google.com"},
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

// virtualEthernetPair represents a Linux pair of virtual Ethernet (veth) devices. Veth devices come in pairs,
// a primary and a peer, representing two sides of a virtual link.
type virtualEthernetPair struct {
	Iface     string
	PeerIface string
}

// Create establishes the virtual Ethernet pair. It removes any existing links of the same name.
func (v *virtualEthernetPair) Create(ctx context.Context) error {
	// Delete any existing links.
	for _, name := range []string{v.Iface, v.PeerIface} {
		// Check if interface 'name' exists.
		if _, err := net.InterfaceByName(name); err == nil {
			testing.ContextLogf(ctx, "Deleting existing interface %s", name)
			if err = testexec.CommandContext(ctx, "ip", "link", "del", name).Run(); err != nil {
				return errors.Errorf("failed to delete existing link %q", name)
			}
		}
	}

	// Create veth pair.
	if err := testexec.CommandContext(ctx, "ip", "link", "add", v.Iface, "type", "veth", "peer", "name", v.PeerIface).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "failed to add veth interfaces %q/%q", v.Iface, v.PeerIface)
	}

	return nil
}

// Delete deletes the virtual link.
func (v *virtualEthernetPair) Delete(ctx context.Context) error {
	// Only need to delete one end of the pair.
	if err := testexec.CommandContext(ctx, "ip", "link", "del", v.Iface).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "failed to delete veth iface %q", v.Iface)
	}
	return nil
}

// wiredHostapdConf holds configuration information for creating a wiredHostapdServer.
type wiredHostapdConf struct {
	iface string

	outerAuth string
	innerAuth string
	identity  string
	password  string
	cert      certificate.Certificate

	// Path to which hostapd logs should be written.
	log string
}

// wiredHostapdServer holds information about a started hostapd server, primarily for the 'driver=wired'
// variant.
type wiredHostapdServer struct {
	iface     string
	ctrlIface string
	tmpDir    string
	cmd       *testexec.Cmd
}

// Stop cleans up any resources and kills the hostapd server.
func (s *wiredHostapdServer) Stop(ctx context.Context) error {
	if err := s.cmd.Kill(); err != nil {
		return errors.Wrap(err, "failed to kill hostapd")
	}
	if err := os.RemoveAll(s.tmpDir); err != nil {
		return errors.Wrapf(err, "failed to clean up tmp dir: %s", s.tmpDir)
	}
	return nil
}

// CliCmd runs a hostapd command via hostapd_cli. Returns combined stdout/stderr for success or error.
func (s *wiredHostapdServer) CliCmd(ctx context.Context, args ...string) (string, error) {
	cliArgs := append([]string{"-p", s.ctrlIface, "-i", s.iface}, args...)
	out, err := testexec.CommandContext(ctx, "hostapd_cli", cliArgs...).CombinedOutput()
	if err != nil {
		return string(out), errors.Wrapf(err, "hostapd_cli failed, args: %v", args)
	}

	return string(out), nil
}

// startHostapdServer starts up a hostapd instance, for wired authentication. The caller should call
// wiredHostapdServer.Stop() on the returned server when finished.
func startHostapdServer(ctx context.Context, conf wiredHostapdConf) (*wiredHostapdServer, error) {
	succeeded := false
	server := wiredHostapdServer{
		iface: conf.iface,
	}

	var err error
	if server.tmpDir, err = ioutil.TempDir("", ""); err != nil {
		return nil, errors.Wrap(err, "failed to create temp dir")
	}
	defer func() {
		if !succeeded {
			if err := os.RemoveAll(server.tmpDir); err != nil {
				testing.ContextLogf(ctx, "Failed to clean up dir %s, %v", server.tmpDir, err)
			}
		}
	}()

	// NB: 'eap_user' format is not well documented. The '[2]' indicates
	// phase 2 (i.e., inner).
	eapUser := fmt.Sprintf(`* %s
"%s" %s "%s" [2]
`, conf.outerAuth, conf.identity, conf.innerAuth, conf.password)

	serverCertPath := filepath.Join(server.tmpDir, "cert")
	privateKeyPath := filepath.Join(server.tmpDir, "private_key")
	eapUserFilePath := filepath.Join(server.tmpDir, "eap_user")
	caCertPath := filepath.Join(server.tmpDir, "ca_cert")
	confPath := filepath.Join(server.tmpDir, "hostapd.conf")
	server.ctrlIface = filepath.Join(server.tmpDir, "hostapd.ctrl")

	confContents := fmt.Sprintf(`driver=wired
interface=%s
ctrl_interface=%s
server_cert=%s
private_key=%s
eap_user_file=%s
ca_cert=%s
eap_server=1
ieee8021x=1
eapol_version=2
`, server.iface, server.ctrlIface, serverCertPath, privateKeyPath, eapUserFilePath, caCertPath)

	for _, p := range []struct {
		path     string
		contents string
	}{
		{confPath, confContents},
		{serverCertPath, conf.cert.Cert},
		{privateKeyPath, conf.cert.PrivateKey},
		{eapUserFilePath, eapUser},
		{caCertPath, conf.cert.CACert},
	} {
		if err := ioutil.WriteFile(p.path, []byte(p.contents), 0644); err != nil {
			return nil, errors.Wrapf(err, "failed to write file %q", p.path)
		}
	}

	// Bring up the hostapd link.
	if err := testexec.CommandContext(ctx, "ip", "link", "set", server.iface, "up").Run(testexec.DumpLogOnError); err != nil {
		return nil, errors.Wrap(err, "could not bring up hostapd veth")
	}

	server.cmd = testexec.CommandContext(ctx, "hostapd", "-dd", "-t", "-f", conf.log, confPath)
	if err := server.cmd.Start(); err != nil {
		return nil, errors.Wrap(err, "failed to start hostapd")
	}

	succeeded = true
	return &server, nil
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
		// The default group MAC address to which EAP challenges are sent, absent any prior knowledge
		// of a specific client on the link -- part of the Link Layer Discovery Protocol (LLDP), IEEE
		// 802.1AB.
		nearestMAC = "01:80:c2:00:00:03"
		vethIface  = "test_ethernet"
		// NB: Shill explicitly avoids managing interfaces whose name is prefixed with 'veth'.
		hostapdIface    = "veth_test"
		identity        = "testuser"
		password        = "password"
		eapProfileEntry = "etherneteap_all"
	)

	param := s.Param().(testParameters)
	cert := certificate.GetTestCertificate()

	m, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed creating shill manager proxy: ", err)
	}

	// Clear out all existing Ethernet EAP properties, to avoid pollination of service Properties from one
	// test to the next.
	if profiles, err := m.GetProfiles(ctx); err != nil {
		s.Fatal("Failed to get profiles: ", err)
	} else {
		for _, path := range profiles {
			if p, err := shill.NewProfile(ctx, path); err != nil {
				s.Fatalf("Failed to get profile at path %q: %v", path, err)
			} else {
				// Ignore errors, since a clean profile may not have this entry.
				p.DeleteEntry(ctx, eapProfileEntry)
			}
		}
	}

	// Clear test profile, in case it exists, but ignore errors.
	m.PopProfile(ctx, testProfile)
	m.RemoveProfile(ctx, testProfile)
	if _, err := m.CreateProfile(ctx, testProfile); err != nil {
		s.Fatal("Failed to create profile: ", err)
	}
	testProfilePath, err := m.PushProfile(ctx, testProfile)
	if err != nil {
		s.Fatal("Failed to push profile: ", err)
	}
	defer func() {
		if err := m.PopProfile(ctx, testProfile); err != nil {
			s.Logf("Failed to pop profile %q: %v", testProfile, err)
		} else if err = m.RemoveProfile(ctx, testProfile); err != nil {
			s.Logf("Failed to remove profile %q: %v", testProfile, err)
		}
	}()

	// Prepare virtual ethernet link.
	veth := virtualEthernetPair{
		Iface:     vethIface,
		PeerIface: hostapdIface,
	}
	if err := veth.Create(ctx); err != nil {
		s.Fatal("Failed to setup veth: ", err)
	}
	defer func() {
		if err := veth.Delete(ctx); err != nil {
			s.Error("Failed to cleanup veth: ", err)
		}
	}()

	s.Log("Waiting for Ethernet Device")
	d, err := m.WaitForDeviceByName(ctx, veth.Iface, time.Second*3)
	if err != nil {
		s.Fatal("Failed to find veth device managed by Shill: ", err)
	}

	waitForDeviceProperty := func(prop string, expected bool) error {
		pw, err := d.CreateWatcher(ctx)
		if err != nil {
			s.Fatal("Failed to create a PropertiesWatcher: ", err)
		}
		defer pw.Close(ctx)

		// Keep a limited timeout for the property-watch.
		tctx, cancel := context.WithTimeout(ctx, time.Second*15)
		defer cancel()
		for {
			props, err := d.GetProperties(ctx)
			if err != nil {
				s.Fatal("Failed to get device properties: ", err)
			}

			if v, err := props.GetBool(prop); err != nil {
				s.Fatalf("Missing Device property %q: %v", prop, err)
			} else if v == expected {
				return nil
			}

			if _, err := pw.WaitAll(tctx, prop); err != nil {
				return errors.Wrapf(err, "failed waiting for Device property %q==%t", prop, expected)
			}
		}
	}

	s.Log("Ensure EAP is not detected yet")
	if err := waitForDeviceProperty(shill.DevicePropertyEapDetected, false); err != nil {
		s.Fatal("Failed to check initial EAP stat: ", err)
	}

	s.Log("Starting hostapd server")
	// Hostapd performs as the authentication server.
	server, err := startHostapdServer(ctx, wiredHostapdConf{
		iface:     veth.PeerIface,
		outerAuth: param.outerAuth,
		innerAuth: param.innerAuth,
		identity:  identity,
		password:  password,
		cert:      cert,
		log:       filepath.Join(s.OutDir(), "hostapd.log"),
	})
	if err != nil {
		s.Fatal("Failed to start hostapd: ", err)
	}
	defer func() {
		if err := server.Stop(ctx); err != nil {
			s.Error("Failed to stop hostapd: ", err)
		}
	}()

	s.Log("Initiating EAP from hostapd")
	var out string
	// Poll because we didn't guarantee the hostapd server has finished starting up (e.g., establishing
	// the control socket).
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		out, err = server.CliCmd(ctx, "new_sta", nearestMAC)
		return err
	}, &testing.PollOptions{Timeout: 3 * time.Second}); err != nil {
		s.Fatalf("new_sta failed, output %q, err %v", out, err)
	}

	s.Log("Waiting for EAP detection")
	if err := waitForDeviceProperty(shill.DevicePropertyEapDetected, true); err != nil {
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
	if _, err := m.ConfigureServiceForProfile(ctx, testProfilePath, eap); err != nil {
		s.Fatal("Failed to configure service: ", err)
	}

	var vethMac string
	if i, err := net.InterfaceByName(veth.Iface); err != nil {
		s.Fatal("Failed to get interface info: ", err)
	} else {
		vethMac = i.HardwareAddr.String()
	}
	// Log hostapd status to file, with increasing index.
	index := 0
	checkSTAStatus := func(expectLine string) error {
		// Avoid shadow assignment to parent context's 'err'.
		var e error

		defer func() {
			index++
		}()

		var out string
		if out, e = server.CliCmd(ctx, "sta", vethMac); e != nil {
			s.Log("hostapd_cli output: ", out)
			s.Fatal("Failed to query STA: ", e)
		}

		// Stash output for analysis.
		writeFile := func(path string, contents string) {
			if e := ioutil.WriteFile(path, []byte(contents), 0644); e != nil {
				s.Fatalf("Failed to write file %q, %v", path, e)
			}
		}
		writeFile(filepath.Join(s.OutDir(), fmt.Sprintf("hostapd_auth_%d.txt", index)), out)

		for _, line := range strings.Split(out, "\n") {
			if line == expectLine {
				return nil
			}
		}

		return errors.Errorf("hostapd auth status %q not found", expectLine)
	}

	s.Log("Waiting for Ethernet EAP auth")
	err = waitForDeviceProperty(shill.DevicePropertyEapCompleted, true)
	// Check in with hostapd in either failure or success.
	err1 := checkSTAStatus("dot1xAuthAuthSuccessesWhileAuthenticating=1")
	if err != nil {
		s.Fatal("Failed to complete EAP: ", err)
	} else if err1 != nil {
		s.Fatal("Server failed to authorize: ", err1)
	}

	// Popping the test profile (and EAP credentials) should initiate log-off.
	if err := m.PopProfile(ctx, testProfile); err != nil {
		s.Fatal("Failed to pop profile: ", err)
	}

	s.Log("Waiting for Ethernet EAP logoff")
	err = waitForDeviceProperty(shill.DevicePropertyEapCompleted, false)
	// Check in with hostapd in either failure or success.
	err1 = checkSTAStatus("dot1xAuthAuthEapLogoffWhileAuthenticated=1")
	if err != nil {
		s.Fatal("Failed to deauth EAP: ", err)
	} else if err1 != nil {
		s.Fatal("Server failed to logoff: ", err1)
	}

	// Push the test profile back, and we should authenticate again.
	if _, err := m.PushProfile(ctx, testProfile); err != nil {
		s.Fatal("Failed to push profile: ", err)
	}

	// NB: the EAP server is not notified that we've re-established our credentials, so we're waiting on
	// the server state machine (RFC 3748 4.3, Retransmission Behavior) to retry auth. This can take a few
	// seconds.
	s.Log("Waiting for Ethernet EAP reauth")
	err = waitForDeviceProperty(shill.DevicePropertyEapCompleted, true)
	// Check in with hostapd in either failure or success.
	err1 = checkSTAStatus("dot1xAuthAuthSuccessesWhileAuthenticating=1")
	if err != nil {
		s.Fatal("Failed to reauth: ", err)
	} else if err1 != nil {
		s.Fatal("Server failed to reauth: ", err1)
	}
}
