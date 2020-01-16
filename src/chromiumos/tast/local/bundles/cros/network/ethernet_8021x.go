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

const (
	// The default group MAC address to which EAP challenges are sent,
	// absent any prior knowledge of a specific client on the link.
	nearestMAC = "01:80:c2:00:00:03"
	vethIface  = "test_ethernet"
	// NB: Shill explicitly avoids managing interfaces whose name
	// is prefixed with 'veth'.
	hostapIface = "veth_test"
	identity    = "testuser"
	password    = "password"
)

// Ethernet8021X tests 802.1X over Ethernet, using a virtual Ethernet pair.
// For the authentication server, use the EAP server built into hostapd.
// Note that we only test that authentication completes successfully; we don't,
// e.g., enable a bridge and DHCP server, so the client never actually gets an
// IP address.
func Ethernet8021X(ctx context.Context, s *testing.State) {
	param := s.Param().(testParameters)

	// NB: 'eap_user' format is not well documented. The '[2]' indicates
	// phase 2 (i.e., inner).
	eapUser := fmt.Sprintf(`* %s
"%s" %s "%s" [2]
`, param.outerAuth, identity, param.innerAuth, password)

	dir, err := ioutil.TempDir("", "")
	if err != nil {
		s.Fatal("Failed to create temp dir: ", err)
	}
	defer func() {
		if err := os.RemoveAll(dir); err != nil {
			s.Errorf("Failed to clean up dir %s, %v", dir, err)
		}
	}()

	serverCertPath := filepath.Join(dir, "cert")
	privateKeyPath := filepath.Join(dir, "private_key")
	eapUserFilePath := filepath.Join(dir, "eap_user")
	caCertPath := filepath.Join(dir, "ca_cert")
	hostapdConfPath := filepath.Join(dir, "hostapd.conf")
	hostapdCtrlIfrace := filepath.Join(dir, "hostapd.ctrl")

	hostapdConf := fmt.Sprintf(`driver=wired
interface=%s
ctrl_interface=%s
server_cert=%s
private_key=%s
eap_user_file=%s
ca_cert=%s
eap_server=1
ieee8021x=1

eapol_version=2
`, hostapIface, hostapdCtrlIfrace, serverCertPath, privateKeyPath, eapUserFilePath, caCertPath)

	writeFile := func(path string, contents string) {
		if err := ioutil.WriteFile(path, []byte(contents), 0644); err != nil {
			s.Fatalf("Failed to write file %q, %v", path, err)
		}
	}

	cert := certificate.GetTestCertificate()

	writeFile(hostapdConfPath, hostapdConf)
	writeFile(serverCertPath, cert.Cert)
	writeFile(privateKeyPath, cert.PrivateKey)
	writeFile(eapUserFilePath, eapUser)
	writeFile(caCertPath, cert.CACert)

	// Delete any existing link. Ignore failures, since we only expect an
	// existing link if prior tests didn't clean up.
	_ = testexec.CommandContext(ctx, "ip", "link", "del", vethIface).Run()

	// Create veth pair.
	if err := testexec.CommandContext(ctx, "ip", "link", "add", vethIface, "type", "veth", "peer", "name", hostapIface).Run(testexec.DumpLogOnError); err != nil {
		s.Fatalf("Could not add veth interfaces %s/%s", vethIface, hostapIface)
	}
	defer func() {
		// Clean up veth pair.
		if err := testexec.CommandContext(ctx, "ip", "link", "del", vethIface).Run(testexec.DumpLogOnError); err != nil {
			s.Error("Could not delete veth: ", err)
		}
	}()

	var vethMac string
	if i, err := net.InterfaceByName(vethIface); err != nil {
		s.Fatal("Failed to get interface info: ", err)
	} else {
		vethMac = i.HardwareAddr.String()
	}

	// Bring up the hostapd-side link.
	if err := testexec.CommandContext(ctx, "ip", "link", "set", hostapIface, "up").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Could not bring up veth: ", err)
	}

	apCmd := testexec.CommandContext(ctx, "hostapd", "-dd", "-f", filepath.Join(s.OutDir(), "hostapd.log"), hostapdConfPath)

	if err := apCmd.Start(); err != nil {
		s.Fatal("Failed to start hostapd: ", err)
	}

	defer func() {
		if err := apCmd.Kill(); err != nil {
			s.Error("Failed to kill hostapd: ", err)
		}
	}()

	m, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed creating shill manager proxy: ", err)
	}

	// Clear out all existing Ethernet EAP properties.
	if profiles, err := m.GetProfiles(ctx); err != nil {
		s.Fatal("Failed to get profiles: ", err)
	} else {
		for _, path := range profiles {
			if p, err := shill.NewProfile(ctx, path); err != nil {
				s.Fatalf("Failed to get profile at path %q: %v", path, err)
			} else {
				// Ignore errors, as the profile may not have this entry.
				_ = p.DeleteEntry(ctx, "etherneteap_all")
			}
		}
	}

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
	var output string
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		out, err := testexec.CommandContext(ctx, "hostapd_cli", "-p", hostapdCtrlIfrace, "-i", hostapIface, "new_sta", nearestMAC).CombinedOutput()
		output = string(out)
		return err
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		s.Fatalf("hostapd_cli new_sta failed, output %q, err %v", output, err)
	}

	s.Log("Waiting for Ethernet Device")
	d, err := m.WaitForDeviceByName(ctx, vethIface, time.Second*3)
	if err != nil {
		s.Fatal("Failed to find veth device managed by Shill: ", err)
	}

	s.Log("Waiting for Ethernet Service")
	if _, err := m.WaitForAnyServiceProperties(ctx, map[string]interface{}{
		shill.ServicePropertyDevice: dbus.ObjectPath(d.String()),
	}, time.Second*20); err != nil {
		s.Fatal("Failed to find matching service: ", err)
	}

	pw, err := d.CreateWatcher(ctx)
	if err != nil {
		s.Fatal("Failed to create a PropertiesWatcher: ", err)
	}
	defer pw.Close(ctx)

	eapStatus := func() (bool, bool) {
		props, err := d.GetProperties(ctx)
		if err != nil {
			s.Fatal("Failed to get device properties: ", err)
		}

		completed, err := props.GetBool("EapAuthenticationCompleted")
		if err != nil {
			s.Fatal("Failed to get Completed: ", err)
		}
		detected, err := props.GetBool("EapAuthenticatorDetected")
		if err != nil {
			s.Fatal("Failed to get Detected: ", err)
		}

		return detected, completed
	}

	// Keep a limited timeout for the property-watch.
	tctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()
	s.Log("Waiting for Ethernet EAP status")
	for {
		detected, completed := eapStatus()
		if detected && completed {
			// Success!
			break
		}

		if _, err := pw.WaitAll(tctx, "EapAuthenticationCompleted"); err != nil {
			s.Error("Waiting for EAP completion: ", err)
			// We failed, but let's check in with hostapd too.
			break
		}
	}

	out, err := testexec.CommandContext(ctx, "hostapd_cli", "-p", hostapdCtrlIfrace, "-i", hostapIface, "sta", vethMac).Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to query STA: ", err)
	}

	// Stash output for analysis.
	writeFile(filepath.Join(s.OutDir(), "hostapd_auth_status.txt"), string(out))

	for _, line := range strings.Split(string(out), "\n") {
		// TODO: figure out if that's all expect in flags. Otherwise,
		// parse this more carefully.
		if line == "flags=[AUTHORIZED]" {
			// That's all, folks!
			return
		}
	}
	// If we got here, hostapd didn't authorize us.
	s.Fatal("Hostapd didn't authorize us")
}
