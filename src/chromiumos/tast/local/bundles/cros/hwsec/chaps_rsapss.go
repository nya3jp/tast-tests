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
		SoftwareDeps: []string{"chrome", "tpm2"},
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

	// Remove all keys/certs, if any at the end. i.e. Cleanup after ourselves.
	defer r.Run(ctx, "sh", "-c", "rm -f "+hwsec.Pkcs11Scratchpad+"/testkey* "+hwsec.Pkcs11Scratchpad+"/testfile*")
	// Note: This test expects a clean /tmp/ (Pkcs11Scratchpad) in the sense that there should be no files prefixed by testkey or testfile when the test start.
	// Note: Also, this test expects a clean keystore, in the sense that there should be no object with the same ID as those used by this test.

	// Create the test file
	const testfile1 = "/tmp/testfile1.txt"
	if err = ioutil.WriteFile(testfile1, []byte("test1"), 0644); err != nil {
		s.Fatal("Failed to write " + testfile1)
	}
	const testfile2 = "/tmp/testfile2.txt"
	if err = ioutil.WriteFile(testfile2, []byte("test2"), 0644); err != nil {
		s.Fatal("Failed to write " + testfile1)
	}

	// Create the software-generated, then imported key
	importedKey, err := pkcs11.Pkcs11CreateRsaSoftwareKey(ctx, utility, "", "testkey1", "111111")
	if err != nil {
		s.Fatal("Failed to create software key: ", err)
	}
	defer func() {
		if err := pkcs11.Pkcs11DestroyKey(ctx, importedKey); err != nil {
			s.Fatal("Failed to clean up software key: ", err)
		}
	}()

	// Create the TPM generated key
	generatedKey, err := pkcs11.Pkcs11CreateRsaGeneratedKey(ctx, utility, "", "testkey2", "222222")
	if err != nil {
		s.Fatal("Failed to create generated key: ", err)
	}
	defer func() {
		if err := pkcs11.Pkcs11DestroyKey(ctx, generatedKey); err != nil {
			s.Fatal("Failed to clean up generated key: ", err)
		}
	}()

	keys := []hwsec.Pkcs11KeyInfo{importedKey, generatedKey}

	// Test the various keys
	for _, k := range keys {
		// Test the various mechanisms
		for _, m := range []hwsec.Pkcs11MechanismInfo{pkcs11.Pkcs11SHA1RSAPKCSPSS(), pkcs11.Pkcs11SHA256RSAPKCSPSS(), pkcs11.Pkcs11SHA1RSAPKCSPSSGeneric(), pkcs11.Pkcs11SHA256RSAPKCSPSSGeneric()} {
			if err = pkcs11.Pkcs11SignVerify(ctx, k, testfile1, testfile2, m); err != nil {
				s.Fatal("SignVerify failed: ", err)
			}
		}
	}
}
