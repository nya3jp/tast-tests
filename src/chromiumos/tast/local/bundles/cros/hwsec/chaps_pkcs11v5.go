// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"io/ioutil"

	libhwsec "chromiumos/tast/common/hwsec"
	libhwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ChapsPKCS11V5,
		Desc: "Verifies PKCS#1 v1.5 works with RSA keys (sign, verify, encrypt, decrypt) in chaps",
		Attr: []string{"informational"},
		Contacts: []string{
			"zuan@chromium.org",
		},
		SoftwareDeps: []string{"chrome"},
	})
}

func ChapsPKCS11V5(ctx context.Context, s *testing.State) {
	// Create the helper for excuting command.
	helper, err := libhwseclocal.NewHelperLocal()
	if err != nil {
		s.Fatal("Error creating helper")
	}

	// Create the utility for interfacing attestation/cryptohome.
	utility, err := libhwsec.NewUtility(ctx, helper, libhwsec.CryptohomeBinaryType)
	if err != nil {
		s.Fatal("Utilty creation error: ", err)
	}

	// Remove all previous keys/certs, if any.
	_, err = helper.RunShell(ctx, "rm -f /tmp/testkey* /tmp/testfile*")

	// Get the system token slot.
	slot, err := utility.GetTokenForUser("")
	if err != nil {
		s.Fatal("System token is unavailable: ", err)
	}

	// Remove objects that may interfere (if any) that is in the key store.
	if err = libhwsec.Pkcs11ClearObject(ctx, helper, slot, "aaaaaa", "privkey"); err != nil {
		s.Fatal("Unable to clear PKCS#11 private keys: ", err)
	}
	if err = libhwsec.Pkcs11ClearObject(ctx, helper, slot, "aaaaaa", "pubkey"); err != nil {
		s.Fatal("Unable to clear PKCS#11 private keys: ", err)
	}
	if err = libhwsec.Pkcs11ClearObject(ctx, helper, slot, "aaaaaa", "cert"); err != nil {
		s.Fatal("Unable to clear PKCS#11 certificates: ", err)
	}

	// Create the software key
	softwareKey, err := libhwsec.Pkcs11CreateRsaSoftwareKey(ctx, helper, utility, "", "testkey1", "aaaaaa")
	if err != nil {
		s.Fatal("Failed to create software key: ", err)
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

	// Test the various mechanisms
	if err = libhwsec.Pkcs11SignVerify(ctx, helper, softwareKey, testfile1, testfile2, libhwsec.Pkcs11SHA1RSAPKCS()); err != nil {
		s.Fatal("SignVerify failed: ", err)
	}

	if err = libhwsec.Pkcs11SignVerify(ctx, helper, softwareKey, testfile1, testfile2, libhwsec.Pkcs11SHA256RSAPKCS()); err != nil {
		s.Fatal("SignVerify failed: ", err)
	}

	// Remove objects that may interfere (if any) that is in the key store.
	if err = libhwsec.Pkcs11ClearObject(ctx, helper, slot, "bbbbbb", "privkey"); err != nil {
		s.Fatal("Unable to clear PKCS#11 private keys: ", err)
	}
	if err = libhwsec.Pkcs11ClearObject(ctx, helper, slot, "bbbbbb", "pubkey"); err != nil {
		s.Fatal("Unable to clear PKCS#11 private keys: ", err)
	}
	if err = libhwsec.Pkcs11ClearObject(ctx, helper, slot, "bbbbbb", "cert"); err != nil {
		s.Fatal("Unable to clear PKCS#11 certificates: ", err)
	}

	// Create the generated key
	generatedKey, err := libhwsec.Pkcs11CreateRsaGeneratedKey(ctx, helper, utility, "", "testkey2", "bbbbbb")
	if err != nil {
		s.Fatal("Failed to create generated key: ", err)
	}

	// Test the various mechanisms
	if err = libhwsec.Pkcs11SignVerify(ctx, helper, generatedKey, testfile1, testfile2, libhwsec.Pkcs11SHA1RSAPKCS()); err != nil {
		s.Fatal("SignVerify failed: ", err)
	}

	if err = libhwsec.Pkcs11SignVerify(ctx, helper, generatedKey, testfile1, testfile2, libhwsec.Pkcs11SHA256RSAPKCS()); err != nil {
		s.Fatal("SignVerify failed: ", err)
	}
}
