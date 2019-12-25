// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"fmt"
	"io/ioutil"

	"chromiumos/tast/common/hwsec"
	libhwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ChapsPKCS1V15,
		Desc: "Verifies PKCS#1 v1.5 works with RSA keys (sign, verify, encrypt, decrypt) in chaps",
		Attr: []string{"informational"},
		Contacts: []string{
			"zuan@chromium.org",
		},
		SoftwareDeps: []string{"chrome"},
	})
}

func ChapsPKCS1V15(ctx context.Context, s *testing.State) {
	r, err := libhwseclocal.NewCmdRunner()
	if err != nil {
		s.Fatal("CmdRunner creation error: ", err)
	}

	utility, err := hwsec.NewUtilityCryptohomeBinary(r)
	if err != nil {
		s.Fatal("Cryptohome Utilty creation error: ", err)
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
	importedKey, err := pkcs11.Pkcs11CreateRsaSoftwareKey(ctx, utility, "", "testkey1", "aaaaaa", false, true)
	if err != nil {
		s.Fatal("Failed to create software key: ", err)
	}
	defer func() {
		if err := pkcs11.Pkcs11DestroyKey(ctx, importedKey); err != nil {
			s.Fatal("Failed to clean up software key: ", err)
		}
	}()

	// Create the software-generated, then imported as software-backed key
	softwareKey, err := pkcs11.Pkcs11CreateRsaSoftwareKey(ctx, utility, "", "testkey2", "bbbbbb", true, true)
	if err != nil {
		s.Fatal("Failed to create software key: ", err)
	}
	defer func() {
		if err := pkcs11.Pkcs11DestroyKey(ctx, softwareKey); err != nil {
			s.Fatal("Failed to clean up software key: ", err)
		}
	}()

	// Create the TPM generated key
	generatedKey, err := pkcs11.Pkcs11CreateRsaGeneratedKey(ctx, utility, "", "testkey3", "cccccc")
	if err != nil {
		s.Fatal("Failed to create generated key: ", err)
	}
	defer func() {
		if err := pkcs11.Pkcs11DestroyKey(ctx, generatedKey); err != nil {
			s.Fatal("Failed to clean up generated key: ", err)
		}
	}()

	keys := []hwsec.Pkcs11KeyInfo{importedKey, softwareKey, generatedKey}

	// Create a copy of software key for every key.
	var copiedKeys []hwsec.Pkcs11KeyInfo
	for i, k := range keys {
		copiedKey, _, err := pkcs11.Pkcs11CreateCopiedKey(ctx, k, fmt.Sprintf("C0B1%02X", i), map[string]string{})
		if err != nil {
			s.Fatal("Failed to copy key: ", err)
		}
		copiedKeys = append(copiedKeys, copiedKey)
	}
	defer func() {
		for _, k := range copiedKeys {
			if err := pkcs11.Pkcs11DestroyKey(ctx, k); err != nil {
				s.Fatal("Failed to clean up copied key: ", err)
			}
		}
	}()

	keys = append(keys, copiedKeys...)

	// Test the various keys
	for _, k := range keys {
		// Test the various mechanisms
		for _, m := range []hwsec.Pkcs11MechanismInfo{pkcs11.Pkcs11SHA1RSAPKCS(), pkcs11.Pkcs11SHA256RSAPKCS()} {
			if err = pkcs11.Pkcs11SignVerify(ctx, k, testfile1, testfile2, m); err != nil {
				s.Fatal("SignVerify failed: ", err)
			}
		}
	}
}
