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
		Func:     Ethernet8021X,
		Desc:     "Verifies we can authenticate Ethernet via 802.1X",
		Contacts: []string{"briannorris@google.com", "cros-networking@google.com"},
		Attr:     []string{"group:mainline", "informational"},

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
	vethIface = "test_ethernet"
	// NB: Shill explicitly avoids managing interfaces whose name
	// is prefixed with 'veth'.
	hostapIface = "veth_test"
	identity    = "testuser"
	password    = "password"

	// Test certificates borrowed from Autotest
	// (client/common_lib/cros/site_eap_certs.py). They are only for test
	// usage.
	cert = `-----BEGIN CERTIFICATE-----
MIIDYTCCAsqgAwIBAgIDEAADMA0GCSqGSIb3DQEBBAUAMG8xCzAJBgNVBAYTAlVT
MRMwEQYDVQQIEwpDYWxpZm9ybmlhMRYwFAYDVQQHEw1Nb3VudGFpbiBWaWV3MTMw
MQYDVQQDEypjaHJvbWVsYWItd2lmaS10ZXN0YmVkLXJvb3QubXR2Lmdvb2dsZS5j
b20wHhcNMTIwNDI2MDE0OTM1WhcNMjIwNDI0MDE0OTM1WjBxMQswCQYDVQQGEwJV
UzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMNTW91bnRhaW4gVmlldzE1
MDMGA1UEAxMsY2hyb21lbGFiLXdpZmktdGVzdGJlZC1zZXJ2ZXIubXR2Lmdvb2ds
ZS5jb20wgZ8wDQYJKoZIhvcNAQEBBQADgY0AMIGJAoGBAKPqAb1K14pPSXjbMPGy
pSzuXtP/1++oKW/gwBHfy/8+D1tlTCEQk29qEv2Mz48JV1iI7zhVV+9gErS5b+En
79yfkCyMVt1/yEsrXofn1aBSd7X5bcQJwnXkozvJGeyWyrHwAnCvHJoty2gAuqDO
F2WFDRXrTC/dbw0n3j+WzH3tAgMBAAGjggEHMIIBAzAJBgNVHRMEAjAAMBEGCWCG
SAGG+EIBAQQEAwIGQDAdBgNVHQ4EFgQUgWAlu2RX2hiaU+KuOdLxjwLxe3QwgaEG
A1UdIwSBmTCBloAUMmchjZGLyuPSX1Yj6unKs/mslD+hc6RxMG8xCzAJBgNVBAYT
AlVTMRMwEQYDVQQIEwpDYWxpZm9ybmlhMRYwFAYDVQQHEw1Nb3VudGFpbiBWaWV3
MTMwMQYDVQQDEypjaHJvbWVsYWItd2lmaS10ZXN0YmVkLXJvb3QubXR2Lmdvb2ds
ZS5jb22CCQDZ/zCAdXrBSDALBgNVHQ8EBAMCBaAwEwYDVR0lBAwwCgYIKwYBBQUH
AwEwDQYJKoZIhvcNAQEEBQADgYEAbO2QQf5seheTE0wKyOP4eCMMHmLqzE/nHd4q
pxz4sQZ6D7aCxsTstVWfXDAWvzRxXO/QY57FTXn7F7e3lA9CP+igfOWbxaRoYiCG
cJAaaSpwUE0GWzPP8zTm6f1NtolffQ5QmUE/Wzn6YLD03S+6TLyS4BlaZRu4kFVF
uUsuMKk=
-----END CERTIFICATE-----
`
	privateKey = `-----BEGIN RSA PRIVATE KEY-----
MIICXQIBAAKBgQCj6gG9SteKT0l42zDxsqUs7l7T/9fvqClv4MAR38v/Pg9bZUwh
EJNvahL9jM+PCVdYiO84VVfvYBK0uW/hJ+/cn5AsjFbdf8hLK16H59WgUne1+W3E
CcJ15KM7yRnslsqx8AJwrxyaLctoALqgzhdlhQ0V60wv3W8NJ94/lsx97QIDAQAB
AoGAJYbBdzDXP9b/HygvgGZB4pOAKlD8guWg9vghgPYXogv3QBlk4H0HBA7o4huG
uVyOHrra6a7APxFjtvRtZMLb6uzJqQ0EccwEcHLtSG7rNNlFsylwgMpcwhWrcz/D
nHQo1Pt8Ij0a6byqCYAsct9F/EPq2WM7FK+0Y6wjiHp9+BECQQDWG+QYE06lDLzo
89kbwPLYHB2G9oDmWvqeajIWXqpb2snHVgphJppEDHx0IDby4uJMxkftqlka1DFC
icx3XKLfAkEAw/v/c2td+wtUiDSSm2z+wPOT0ifrY99i77nOm8BOLXhiYvBHBo2q
4fgkxaApo6Lg0wLOkcqnJLpVByBdlwjkswJBALMelDzb8iA8PtI4JjqEueS36K/f
C0krdZ0PxKVYPvcnW0UrIvXRoJ8rPva7eJzL2HxYKRaYO4EpYaiDtY1p70sCQQCY
CK0qRGgrj6aL4vy4Rd16oXpS1VTtrSV7ApEckhoTfAgW6H6wvsWJdo5QIOcsYfY2
uz60KplvDH1ZgeoYeHWxAkBptmY574GnuAW1Jw1G6xWwka6yLs68PfQgslHZiYT7
eOiTwkwmWD7oFjqWUYO1Pw7ipktE1uFPd4KUtwcMbid9
-----END RSA PRIVATE KEY-----
`
	caCert = `-----BEGIN CERTIFICATE-----
MIIDRjCCAq+gAwIBAgIJANn/MIB1esFIMA0GCSqGSIb3DQEBBQUAMG8xCzAJBgNV
BAYTAlVTMRMwEQYDVQQIEwpDYWxpZm9ybmlhMRYwFAYDVQQHEw1Nb3VudGFpbiBW
aWV3MTMwMQYDVQQDEypjaHJvbWVsYWItd2lmaS10ZXN0YmVkLXJvb3QubXR2Lmdv
b2dsZS5jb20wHhcNMTIwNDI2MDE0OTMxWhcNMjIwNDI0MDE0OTMxWjBvMQswCQYD
VQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMNTW91bnRhaW4g
VmlldzEzMDEGA1UEAxMqY2hyb21lbGFiLXdpZmktdGVzdGJlZC1yb290Lm10di5n
b29nbGUuY29tMIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQC7ih4cIp4P5aRu
8ydFu0ggXr0gVLEdIMbHg3hfPjluDzNhbAP400+Vg0zJgfyOJCE8I6qzKMIX4MGD
EKBGADmB68gffQiwkVGr7IwzeR7Qmy5j1M0Ks6HS1V0wLPgDBSnf8HtqRuHU63V4
3mpiW8DltXSbO1QmgtDHLIHhIPukTwIDAQABo4HpMIHmMB0GA1UdDgQWBBQyZyGN
kYvK49JfViPq6cqz+ayUPzCBoQYDVR0jBIGZMIGWgBQyZyGNkYvK49JfViPq6cqz
+ayUP6FzpHEwbzELMAkGA1UEBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAU
BgNVBAcTDU1vdW50YWluIFZpZXcxMzAxBgNVBAMTKmNocm9tZWxhYi13aWZpLXRl
c3RiZWQtcm9vdC5tdHYuZ29vZ2xlLmNvbYIJANn/MIB1esFIMAwGA1UdEwQFMAMB
Af8wEwYDVR0lBAwwCgYIKwYBBQUHAwMwDQYJKoZIhvcNAQEFBQADgYEACy7WcGIZ
NfpnIrdM0TpzYrqkzNEdrdvO32mX4WKrpF2YdhNQ6NMqLJEHjq4iTwMMf1oxUT+X
R2fZba/umMvP8s2RASNKzmozw0GRuK8wzsFYjC/85TwL3Z6d2nzgpBjVtpE5kROY
b6ZSoIDgYwTUgvLrROpy4Uc68PrGnFcCvCE=
-----END CERTIFICATE-----
`
	clientCert = `-----BEGIN CERTIFICATE-----
MIIDRjCCAq+gAwIBAgIJANn/MIB1esFIMA0GCSqGSIb3DQEBBQUAMG8xCzAJBgNV
BAYTAlVTMRMwEQYDVQQIEwpDYWxpZm9ybmlhMRYwFAYDVQQHEw1Nb3VudGFpbiBW
aWV3MTMwMQYDVQQDEypjaHJvbWVsYWItd2lmaS10ZXN0YmVkLXJvb3QubXR2Lmdv
b2dsZS5jb20wHhcNMTIwNDI2MDE0OTMxWhcNMjIwNDI0MDE0OTMxWjBvMQswCQYD
VQQGEwJVUzETMBEGA1UECBMKQ2FsaWZvcm5pYTEWMBQGA1UEBxMNTW91bnRhaW4g
VmlldzEzMDEGA1UEAxMqY2hyb21lbGFiLXdpZmktdGVzdGJlZC1yb290Lm10di5n
b29nbGUuY29tMIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQC7ih4cIp4P5aRu
8ydFu0ggXr0gVLEdIMbHg3hfPjluDzNhbAP400+Vg0zJgfyOJCE8I6qzKMIX4MGD
EKBGADmB68gffQiwkVGr7IwzeR7Qmy5j1M0Ks6HS1V0wLPgDBSnf8HtqRuHU63V4
3mpiW8DltXSbO1QmgtDHLIHhIPukTwIDAQABo4HpMIHmMB0GA1UdDgQWBBQyZyGN
kYvK49JfViPq6cqz+ayUPzCBoQYDVR0jBIGZMIGWgBQyZyGNkYvK49JfViPq6cqz
+ayUP6FzpHEwbzELMAkGA1UEBhMCVVMxEzARBgNVBAgTCkNhbGlmb3JuaWExFjAU
BgNVBAcTDU1vdW50YWluIFZpZXcxMzAxBgNVBAMTKmNocm9tZWxhYi13aWZpLXRl
c3RiZWQtcm9vdC5tdHYuZ29vZ2xlLmNvbYIJANn/MIB1esFIMAwGA1UdEwQFMAMB
Af8wEwYDVR0lBAwwCgYIKwYBBQUHAwMwDQYJKoZIhvcNAQEFBQADgYEACy7WcGIZ
NfpnIrdM0TpzYrqkzNEdrdvO32mX4WKrpF2YdhNQ6NMqLJEHjq4iTwMMf1oxUT+X
R2fZba/umMvP8s2RASNKzmozw0GRuK8wzsFYjC/85TwL3Z6d2nzgpBjVtpE5kROY
b6ZSoIDgYwTUgvLrROpy4Uc68PrGnFcCvCE=
-----END CERTIFICATE-----
`
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

	writeFile(hostapdConfPath, hostapdConf)
	writeFile(serverCertPath, cert)
	writeFile(privateKeyPath, privateKey)
	writeFile(eapUserFilePath, eapUser)
	writeFile(caCertPath, caCert)

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
		shill.ServicePropertyEAPCACertPEM: []string{clientCert},
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
	// Note: in a typical 802.1X environment, the Ethernet bridge would
	// automatically detect the link-presence and initiate EAPOL. Based on
	// experimentation, it appears hostapd doesn't automatically do that
	// for us, so we need to manually initiate.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		out, err := testexec.CommandContext(ctx, "hostapd_cli", "-p", hostapdCtrlIfrace, "-i", hostapIface, "new_sta", vethMac).CombinedOutput()
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
