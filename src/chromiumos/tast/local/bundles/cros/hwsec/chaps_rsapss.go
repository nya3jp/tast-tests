// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"io/ioutil"

	"chromiumos/tast/common/hwsec"
	libhwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ChapsRSAPSS,
		Desc: "Verifies RSA PSS works with RSA keys (sign, verify, encrypt, decrypt) in chaps",
		Attr: []string{"informational"},
		Contacts: []string{
			"zuan@chromium.org",
		},
		SoftwareDeps: []string{"chrome"},
	})
}

func ChapsRSAPSS(ctx context.Context, s *testing.State) {
	r, err := libhwseclocal.NewCmdRunner()
	if err != nil {
		s.Fatal("CmdRunner creation error: ", err)
	}
	utility, err := hwsec.NewUtilityCryptohomeBinary(r)
	if err != nil {
		s.Fatal("Utilty creation error: ", err)
	}
	pkcs11, err := hwsec.NewPkcs11Util(r, utility)
	if err != nil {
		s.Fatal("PKCS#11 Utility creation error: ", err)
	}

	// Remove all previous keys/certs, if any.
	_, err = r.Run(ctx, "sh", "-c", "rm -f /tmp/testkey* /tmp/testfile*")

	// Get the system token slot.
	slot, err := utility.GetTokenForUser(ctx, "")
	if err != nil {
		s.Fatal("System token is unavailable: ", err)
	}

	// Create the test file
	const testfile1 = "/tmp/testfile1.txt"
	if err = ioutil.WriteFile(testfile1, []byte("test1"), 0644); err != nil {
		s.Fatal("Failed to write " + testfile1)
	}
	const testfile2 = "/tmp/testfile2.txt"
	if err = ioutil.WriteFile(testfile2, []byte("test2"), 0644); err != nil {
		s.Fatal("Failed to write " + testfile1)
	}

	// Remove objects that may interfere (if any) that is in the key store.
	if err = pkcs11.Pkcs11ClearObjectOfAllType(ctx, slot, "111111"); err != nil {
		s.Fatal("Unable to clear PKCS#11 object: ", err)
	}

	// Create the software key
	softwareKey, err := pkcs11.Pkcs11CreateRsaSoftwareKey(ctx, utility, "", "testkey1", "111111")
	if err != nil {
		s.Fatal("Failed to create software key: ", err)
	}

	// Test the various mechanisms
	for _, m := range []hwsec.Pkcs11MechanismInfo{pkcs11.Pkcs11SHA1RSAPKCSPSS(), pkcs11.Pkcs11SHA256RSAPKCSPSS(), pkcs11.Pkcs11SHA1RSAPKCSPSSGeneric(), pkcs11.Pkcs11SHA256RSAPKCSPSSGeneric()} {
		if err = pkcs11.Pkcs11SignVerify(ctx, softwareKey, testfile1, testfile2, m); err != nil {
			s.Fatal("SignVerify failed: ", err)
		}
	}

	// Remove objects that may interfere (if any) that is in the key store.
	if err = pkcs11.Pkcs11ClearObjectOfAllType(ctx, slot, "222222"); err != nil {
		s.Fatal("Unable to clear PKCS#11 object: ", err)
	}

	// Create the generated key
	generatedKey, err := pkcs11.Pkcs11CreateRsaGeneratedKey(ctx, utility, "", "testkey2", "222222")
	if err != nil {
		s.Fatal("Failed to create generated key: ", err)
	}

	// Test the various mechanisms
	for _, m := range []hwsec.Pkcs11MechanismInfo{pkcs11.Pkcs11SHA1RSAPKCSPSS(), pkcs11.Pkcs11SHA256RSAPKCSPSS(), pkcs11.Pkcs11SHA1RSAPKCSPSSGeneric(), pkcs11.Pkcs11SHA256RSAPKCSPSSGeneric()} {
		if err = pkcs11.Pkcs11SignVerify(ctx, generatedKey, testfile1, testfile2, m); err != nil {
			s.Fatal("SignVerify failed: ", err)
		}
	}
}
