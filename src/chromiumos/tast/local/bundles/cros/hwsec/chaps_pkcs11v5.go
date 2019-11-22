// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"io/ioutil"

	libhwsec "chromiumos/tast/common/hwsec"
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
	// Create the utility for interfacing attestation/cryptohome.
	utility, err := libhwsec.NewUtility(ctx, s, libhwsec.CryptohomeBinaryType)
	if err != nil {
		s.Fatal("Utilty creation error: ", err)
	}

	// Remove all previous keys/certs, if any.
	_, err = libhwsec.Call(ctx, s, "sh", "-c", "rm -f /tmp/testkey1* /tmp/testfile*")

	// Get the system token slot.
	slot, err := utility.GetTokenForUser("")
	if err != nil {
		s.Fatal("System token is unavailable: ", err)
	}

	// Remove objects that may interfere (if any) that is in the key store.
	if err = libhwsec.Pkcs11ClearObject(ctx, s, slot, "aaaaaa", "privkey"); err != nil {
		s.Fatal("Unable to clear PKCS#11 private keys: ", err)
	}
	if err = libhwsec.Pkcs11ClearObject(ctx, s, slot, "aaaaaa", "cert"); err != nil {
		s.Fatal("Unable to clear PKCS#11 certificates: ", err)
	}

	// Create the keys
	key, err := libhwsec.Pkcs11CreateRsaSoftwareKey(ctx, s, utility, "", "testkey1", "aaaaaa")
	if err != nil {
		s.Fatal("Failed to create key: ", err)
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
	if err = libhwsec.Pkcs11SignVerify(ctx, s, key, testfile1, testfile2, libhwsec.Pkcs11SHA1RSAPKCS()); err != nil {
		s.Fatal("SignVerify failed: ", err)
	}

	if err = libhwsec.Pkcs11SignVerify(ctx, s, key, testfile1, testfile2, libhwsec.Pkcs11SHA256RSAPKCS()); err != nil {
		s.Fatal("SignVerify failed: ", err)
	}
}
