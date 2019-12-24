// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"fmt"
	"strings"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/common/pkcs11"
	libhwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ChapsAttributePolicy,
		Desc: "Verifies Chaps Attribute policy works as intended",
		Attr: []string{"informational"},
		Contacts: []string{
			"zuan@chromium.org",
			"cros-hwsec@chromium.org",
		},
		SoftwareDeps: []string{"chrome"},
	})
}

func checkCannotRead(ctx context.Context, s *testing.State, pkcs11Util *pkcs11.Util, k *pkcs11.KeyInfo, attributeName string) {
	res, _, err := k.GetObjectAttribute(ctx, pkcs11Util, "privkey", attributeName)
	if err == nil {
		s.Errorf("%q readable when it shouldn't be, got %q", attributeName, res)
	}
	if err.Error() == "CKR_ATTRIBUTE_TYPE_INVALID" {
		s.Log(attributeName + " doesn't exist.")
	} else if err.Error() == "CKR_ATTRIBUTE_SENSITIVE" {
		s.Log(attributeName + " is unreadable (as it should be).")
	} else {
		s.Errorf("Incorrect error code %q when testing if %q is readable", err.Error(), attributeName)
	}
}

func checkCannotWrite(ctx context.Context, s *testing.State, pkcs11Util *pkcs11.Util, k *pkcs11.KeyInfo, attributeName string) {
	_, err := k.SetObjectAttribute(ctx, pkcs11Util, "privkey", attributeName, "01")
	if err == nil {
		s.Errorf("%q writable when it shouldn't be", attributeName)
	}
	if err.Error() != "CKR_ATTRIBUTE_READ_ONLY" {
		s.Errorf("Incorrect error code %q when testing if %q is writable", err.Error(), attributeName)
	}

}

func checkCannotWriteOnCopy(ctx context.Context, s *testing.State, pkcs11Util *pkcs11.Util, k *pkcs11.KeyInfo, attributeName string) {
	attributeMap := map[string]string{}
	attributeMap[attributeName] = "01"
	newKey, msg, err := pkcs11Util.CreateCopiedKey(ctx, k, "BAADF00D", attributeMap)
	if err == nil {
		// Destroy the key that we've accidentally created.
		newKey.DestroyKey(ctx, pkcs11Util)

		// Fail the test because such key should not be created.
		s.Errorf("%q is settable on copy", attributeName)
	}

	if !strings.Contains(msg, "CKR_ATTRIBUTE_READ_ONLY") {
		s.Errorf("Incorrect error message %q when testing if %q is writable on copy", msg, attributeName)
	}
}

func checkKey(ctx context.Context, s *testing.State, pkcs11Util *pkcs11.Util, k *pkcs11.KeyInfo) {
	const IssuerTestValue = "AABBCC"

	// Sanity test that whatever that should be writable and readable should be so. CKA_ISSUER is used here.
	if _, err := k.SetObjectAttribute(ctx, pkcs11Util, "privkey", "CKA_ISSUER", IssuerTestValue); err != nil {
		s.Fatal("Unable to set CKA_ISSUER attribute: ", err)
	}

	// Read it back to check that it's set correctly.
	res, _, err := k.GetObjectAttribute(ctx, pkcs11Util, "privkey", "CKA_ISSUER")
	if err != nil {
		s.Fatal("Unable to get CKA_ISSUER attribute: ", err)
	}
	if res != IssuerTestValue {
		s.Fatalf("CKA_ISSUER is not written correctly. Got %q, want %q", res, IssuerTestValue)
	}

	// None of these should be readable.
	for _, attributeName := range []string{"CKA_PRIME_1", "CKA_VALUE", "kKeyBlobAttribute"} {
		checkCannotRead(ctx, s, pkcs11Util, k, attributeName)
	}

	// None of these should be writable.
	for _, attributeName := range []string{"CKA_ALWAYS_SENSITIVE", "CKA_NEVER_EXTRACTABLE", "CKA_MODULUS", "CKA_EC_PARAMS", "kKeyBlobAttribute"} {
		checkCannotWrite(ctx, s, pkcs11Util, k, attributeName)
	}

	// None of these should be writable at copy time.
	for _, attributeName := range []string{"CKA_TOKEN", "CKA_CLASS", "kKeyBlobAttribute"} {
		checkCannotWriteOnCopy(ctx, s, pkcs11Util, k, attributeName)
	}
}

func ChapsAttributePolicy(ctx context.Context, s *testing.State) {
	r, err := libhwseclocal.NewCmdRunner()
	if err != nil {
		s.Fatal("CmdRunner creation error: ", err)
	}
	utility, err := hwsec.NewUtilityCryptohomeBinary(r)
	if err != nil {
		s.Fatal("Utilty creation error: ", err)
	}
	pkcs11Util, err := pkcs11.NewUtil(ctx, r, utility)
	if err != nil {
		s.Fatal("Failed to create PKCS#11 Utility: ", err)
	}

	// Prepare the scratchpad.
	if _, _, err := pkcs11Util.PrepareScratchpadAndTestFiles(ctx, "/tmp/ChapsAttributePolicyTest"); err != nil {
		s.Fatal("Failed to initialize the scratchpad space: ", err)
	}
	// Remove all keys/certs, if any at the end. i.e. Cleanup after ourselves.
	defer pkcs11Util.CleanupScratchpad(ctx)
	// Note: Also, this test expects a clean keystore, in the sense that there should be no object with the same ID as those used by this test.

	// Create the software-generated, then imported key.
	importedKey, err := pkcs11Util.CreateRSASoftwareKey(ctx, utility, "", "testkey1", "999999")
	if err != nil {
		s.Fatal("Failed to create software key: ", err)
	}
	defer func() {
		if err := importedKey.DestroyKey(ctx, pkcs11Util); err != nil {
			s.Error("Failed to clean up software key: ", err)
		}
	}()

	// Create the TPM generated key.
	generatedKey, err := pkcs11Util.CreateRsaGeneratedKey(ctx, utility, "", "testkey3", "777777")
	if err != nil {
		s.Fatal("Failed to create generated key: ", err)
	}
	defer func() {
		if err := generatedKey.DestroyKey(ctx, pkcs11Util); err != nil {
			s.Error("Failed to clean up generated key: ", err)
		}
	}()

	keys := []*pkcs11.KeyInfo{importedKey, generatedKey}

	// Create a copy of software key for every key.
	var copiedKeys []*pkcs11.KeyInfo
	for i, k := range keys {
		// Note: C0B1%02X format is just to avoid collision with other key ID. C0B1 => closest "hexspeak" for copy.
		copiedKey, _, err := pkcs11Util.CreateCopiedKey(ctx, k, fmt.Sprintf("C0B1%02X", i), map[string]string{})
		if err != nil {
			s.Fatal("Failed to copy key: ", err)
		}
		copiedKeys = append(copiedKeys, copiedKey)
	}
	defer func() {
		for _, k := range copiedKeys {
			if err := k.DestroyKey(ctx, pkcs11Util); err != nil {
				s.Error("Failed to clean up copied key: ", err)
			}
		}
	}()

	keys = append(keys, copiedKeys...)

	// Test the various keys.
	for _, k := range keys {
		checkKey(ctx, s, pkcs11Util, k)
	}
}
