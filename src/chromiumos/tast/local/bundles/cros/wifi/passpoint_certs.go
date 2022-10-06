// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/wifi/passpoint"
	"chromiumos/tast/testing"
)

func init() {
	// Set of tests designed to reproduce Passpoint provisioning interaction
	// of certificates and keys ARC. It is expected for certificates and
	// keys to be added once the Passpoint credentials is provisioned. Once
	// the credentials is removed, the certificates and keys are supposed to
	// be removed as well.
	testing.AddTest(&testing.Test{
		Func: PasspointCerts,
		Desc: "Passpoint network certificates provisioning and removal tests",
		Contacts: []string{
			"jasongustaman@google.com",
			"damiendejean@google.com",
			"cros-networking@google.com",
		},
		Fixture:      "arcBooted",
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"wifi", "shill-wifi", "chrome", "arc"},
		LacrosStatus: testing.LacrosVariantUnneeded,
		Timeout:      7 * time.Minute,
	})
}

// pkcs11Object is a structure that holds PKCS#11 object data.
type pkcs11Object struct {
	class string
	id    string
}

func PasspointCerts(ctx context.Context, s *testing.State) {
	// Fully qualified domain name of the Passpoint credentials.
	// This value matches the domain of the certificate used by the AP, chromiumos/tast/common/crypto/certificate TestCert1().
	const fqdn = "chromelab-wifi-testbed-server.mtv.google.com"
	creds := passpoint.Credentials{
		Domains: []string{fqdn},
		HomeOIs: []uint64{passpoint.HomeOI},
		Auth:    passpoint.AuthTLS,
	}

	// Get ARC handle to provision credentials.
	a := s.FixtValue().(*arc.PreData).ARC

	// Get the current PKCS#11 objects.
	objs, err := getPKCS11Objects(ctx)
	if err != nil {
		s.Fatal("Failed to get PKCS#11 objects: ", err)
	}

	// Run the test twice to ensure that adding the same certificates and
	// private keys work after Passpoint credentials removal.
	for i := 0; i < 2; i++ {
		// Provision Passpoint credentials from ARC.
		config, err := creds.ToAndroidConfig(ctx)
		if err != nil {
			s.Fatal("Failed to create Android's config: ", err)
		}
		if err := a.Command(ctx, "cmd", "wifi", "add-passpoint-config", config).Run(testexec.DumpLogOnError); err != nil {
			s.Fatal("Failed to add Passpoint config from ARC: ", err)
		}

		// Ensure that a certificate and a private key is added. It
		// might take a while for the certificate and key to be added.
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			aObjs, err := getAddedPKCS11Objects(ctx, objs)
			if err != nil {
				return errors.Wrap(err, "failed to get added PKCS#11 objects")
			}
			if len(aObjs) != 2 {
				return errors.Errorf("expected 2 additional objects but got %d", len(aObjs))
			}
			if aObjs[0].id != aObjs[1].id {
				return errors.Errorf("certificate and private key ID does not match %v", aObjs)
			}
			if aObjs[0].class != "Private Key" && aObjs[1].class != "Private Key" {
				return errors.New("expected at least one private key")
			}
			if aObjs[0].class != "Certificate" && aObjs[1].class != "Certificate" {
				return errors.New("expected at least one certificate")
			}
			return nil
		}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
			s.Error("Failed to ensure correct certificate and key added: ", err)
		}

		// Remove Passpoint credentials from ARC.
		if err := a.Command(ctx, "cmd", "wifi", "remove-passpoint-config", fqdn).Run(testexec.DumpLogOnError); err != nil {
			s.Error("Failed to remove Passpoint config from ARC: ", err)
		}

		// Ensure that the added certificate and key are removed.
		aObjs, err := getAddedPKCS11Objects(ctx, objs)
		if err != nil {
			s.Error("Failed to get added PKCS#11 objects: ", err)
		}
		if len(aObjs) != 0 {
			s.Error("Failed to remove certificate or key, still got ", aObjs)
		}
	}
}

// getPKCS11Objects get all PKCS#11 objects from the user slot (slot 1).
func getPKCS11Objects(ctx context.Context) ([]pkcs11Object, error) {
	out, err := testexec.CommandContext(ctx, "pkcs11-tool", "--module", "/usr/lib64/libchaps.so", "--slot", "1", "-O").Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, err
	}

	// Regex to get PKCS#11 object's class, the expected data is as follows:
	// "Certificate Object; type = X.509 cert"
	var rc = regexp.MustCompile(`([a-zA-Z ]*) Object;`)
	// Regex to get PKCS#11 object's ID, the expected data is as follows:
	// " ID:         45ce301d2acd4c11909b34919866053ad96c04b2"
	var ri = regexp.MustCompile(`\s*ID:\s*(.*)`)

	var objs []pkcs11Object
	for _, l := range strings.Split(string(out), "\n") {
		m := rc.FindStringSubmatch(l)
		if len(m) == 2 {
			objs = append(objs, pkcs11Object{class: m[1]})
			continue
		}
		m = ri.FindStringSubmatch(l)
		if len(m) == 2 && len(objs) > 0 {
			objs[len(objs)-1].id = m[1]
		}
	}
	return objs, nil
}

// getAddedPKCS11Objects get all added PKCS#11 objects from the user slot
// (slot 1) by comparing the current available objects and the parameter
// |obj|.
func getAddedPKCS11Objects(ctx context.Context, objs []pkcs11Object) ([]pkcs11Object, error) {
	nObjs, err := getPKCS11Objects(ctx)
	if err != nil {
		return nil, err
	}

	// Get additional objects.
	var aObjs []pkcs11Object
	for _, nobj := range nObjs {
		exist := false
		for _, obj := range objs {
			if nobj == obj {
				exist = true
				break
			}
		}
		if !exist {
			aObjs = append(aObjs, nobj)
		}
	}
	return aObjs, nil
}
