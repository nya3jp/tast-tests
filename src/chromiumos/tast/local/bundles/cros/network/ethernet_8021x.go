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

	"github.com/godbus/dbus"

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

	cert certificate.Certificate

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

	server.cmd = testexec.CommandContext(ctx, "hostapd", "-dd", "-f", conf.log, confPath)
	if err := server.cmd.Start(); err != nil {
		return nil, errors.Wrap(err, "failed to start hostapd")
	}

	succeeded = true
	return &server, nil
}

// Ethernet8021X tests 802.1X over Ethernet, using a virtual Ethernet pair. For the authentication server, use
// the EAP server built into hostapd.
// Note that we only test that authentication completes successfully; we don't, e.g., start up a DHCP server,
// so the client never actually gets an IP address. Real 802.1X-enabled switches would typically have
// additional infrastructure to bridge the client over to the "main" network after authentication; hostapd
// doesn't implement this feature, and we consider that beyond the scope of an "authentication" test.
func Ethernet8021X(ctx context.Context, s *testing.State) {
	const (
		// The default group MAC address to which EAP challenges are sent, absent any prior knowledge
		// of a specific client on the link -- part of the Link Layer Discovery Protocol (LLDP), IEEE
		// 802.1AB.
		nearestMAC = "01:80:c2:00:00:03"
		vethIface  = "test_ethernet"
		// NB: Shill explicitly avoids managing interfaces whose name is prefixed with 'veth'.
		hostapdIface = "veth_test"
		identity     = "testuser"
		password     = "password"
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
				_ = p.DeleteEntry(ctx, "etherneteap_all")
			}
		}
	}

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
	if err := m.ConfigureService(ctx, eap); err != nil {
		s.Fatal("Failed to configure service: ", err)
	}

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

	s.Log("Waiting for Ethernet Device")
	d, err := m.WaitForDeviceByName(ctx, veth.Iface, time.Second*3)
	if err != nil {
		s.Fatal("Failed to find veth device managed by Shill: ", err)
	}

	s.Log("Waiting for Ethernet Service")
	if _, err := m.WaitForAnyServiceProperties(ctx, map[string]interface{}{
		shill.ServicePropertyDevice: dbus.ObjectPath(d.String()),
	}, time.Second*3); err != nil {
		s.Fatal("Failed to find matching service: ", err)
	}

	pw, err := d.CreateWatcher(ctx)
	if err != nil {
		s.Fatal("Failed to create a PropertiesWatcher: ", err)
	}
	defer pw.Close(ctx)

	// Keep a limited timeout for the property-watch.
	tctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()
	s.Log("Waiting for Ethernet EAP status")
	for {
		props, err := d.GetProperties(ctx)
		if err != nil {
			s.Fatal("Failed to get device properties: ", err)
		}

		completed, err := props.GetBool(shill.DevicePropertyEapCompleted)
		if err != nil {
			s.Fatal("Missing EAP Completed property: ", err)
		}
		detected, err := props.GetBool(shill.DevicePropertyEapDetected)
		if err != nil {
			s.Fatal("Missing EAP Detected property: ", err)
		}

		if detected && completed {
			// Success!
			break
		}

		if _, err := pw.WaitAll(tctx, shill.DevicePropertyEapCompleted); err != nil {
			// We failed, but let's check in with hostapd too.
			s.Error("Waiting for EAP completion: ", err)
			break
		}
	}

	var vethMac string
	if i, err := net.InterfaceByName(veth.Iface); err != nil {
		s.Fatal("Failed to get interface info: ", err)
	} else {
		vethMac = i.HardwareAddr.String()
	}

	out, err = server.CliCmd(ctx, "sta", vethMac)
	if err != nil {
		s.Log("hostapd_cli output: ", out)
		s.Fatal("Failed to query STA: ", err)
	}

	// Stash output for analysis.
	writeFile := func(path string, contents string) {
		if err := ioutil.WriteFile(path, []byte(contents), 0644); err != nil {
			s.Fatalf("Failed to write file %q, %v", path, err)
		}
	}
	writeFile(filepath.Join(s.OutDir(), "hostapd_auth_status.txt"), out)

	for _, line := range strings.Split(out, "\n") {
		if line == "dot1xAuthAuthSuccessesWhileAuthenticating=1" {
			// That's all, folks!
			return
		}
	}
	// If we got here, hostapd didn't authorize us.
	s.Fatal("Hostapd didn't authorize us")
}
