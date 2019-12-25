// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"fmt"
	"strings"

	"chromiumos/tast/common/hwsec"
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
		},
		SoftwareDeps: []string{"chrome"},
	})
}

func checkCannotRead(ctx context.Context, s *testing.State, pkcs11 *hwsec.Pkcs11Util, k hwsec.Pkcs11KeyInfo, attributeName string) {
	res, msg, err := pkcs11.Pkcs11GetObjectAttribute(ctx, k, "privkey", attributeName)
	if err == nil {
		s.Fatal(attributeName + " readable when it shouldn't be, got: " + res)
	}
	if strings.Contains(msg, "CKR_ATTRIBUTE_TYPE_INVALID") {
		s.Log(attributeName + " doesn't exist.")
	} else if strings.Contains(msg, "CKR_ATTRIBUTE_SENSITIVE") {
		s.Log(attributeName + " is unreadable (as it should be).")
	} else {
		s.Fatal("Incorrect error message when testing if " + attributeName + " is readable")
	}

}

func checkCannotWrite(ctx context.Context, s *testing.State, pkcs11 *hwsec.Pkcs11Util, k hwsec.Pkcs11KeyInfo, attributeName string) {
	msg, err := pkcs11.Pkcs11SetObjectAttribute(ctx, k, "privkey", attributeName, "01")
	if err == nil {
		s.Fatal(attributeName + " writable when it shouldn't be")
	}
	if !strings.Contains(msg, "CKR_ATTRIBUTE_READ_ONLY") {
		s.Fatal("Incorrect error message when testing if " + attributeName + " is writable")
	}

}

func checkCannotWriteOnCopy(ctx context.Context, s *testing.State, pkcs11 *hwsec.Pkcs11Util, k hwsec.Pkcs11KeyInfo, attributeName string) {
	attributeMap := map[string]string{}
	attributeMap[attributeName] = "01"
	newKey, msg, err := pkcs11.Pkcs11CreateCopiedKey(ctx, k, "BAADF00D", attributeMap)
	if err == nil {
		// Destroy the key that we've accidentally created.
		pkcs11.Pkcs11DestroyKey(ctx, newKey)

		// Fail the test because such key should not be created.
		s.Fatal(attributeName + " is settable on copy.")
	}

	if !strings.Contains(msg, "CKR_ATTRIBUTE_READ_ONLY") {
		s.Fatal("Incorrect error message when testing if " + attributeName + " is writable on copy")
	}
}

func checkKey(ctx context.Context, s *testing.State, pkcs11 *hwsec.Pkcs11Util, k hwsec.Pkcs11KeyInfo) {
	const IssuerTestValue = "AABBCC"

	// Sanity test that whatever that should be writable and readable should be so. CKA_ISSUER is used here.
	if _, err := pkcs11.Pkcs11SetObjectAttribute(ctx, k, "privkey", "CKA_ISSUER", IssuerTestValue); err != nil {
		s.Fatal("Unable to set CKA_ISSUER attribute: ", err)
	}

	// Read it back to check that it's set correctly.
	res, _, err := pkcs11.Pkcs11GetObjectAttribute(ctx, k, "privkey", "CKA_ISSUER")
	if err != nil {
		s.Fatal("Unable to get CKA_ISSUER attribute: ", err)
	}
	if res != IssuerTestValue {
		s.Fatalf("CKA_ISSUER is not written correctly. Expected '%q Got %q", IssuerTestValue, res)
	}

	// None of these should be readable.
	for _, attributeName := range []string{"CKA_PRIME_1", "CKA_VALUE", "kKeyBlobAttribute"} {
		checkCannotRead(ctx, s, pkcs11, k, attributeName)
	}

	// None of these should be writable.
	for _, attributeName := range []string{"CKA_ALWAYS_SENSITIVE", "CKA_NEVER_EXTRACTABLE", "CKA_MODULUS", "CKA_EC_PARAMS", "kKeyBlobAttribute"} {
		checkCannotWrite(ctx, s, pkcs11, k, attributeName)
	}

	// None of these should be writable at copy time.
	for _, attributeName := range []string{"CKA_TOKEN", "CKA_CLASS", "kKeyBlobAttribute"} {
		checkCannotWriteOnCopy(ctx, s, pkcs11, k, attributeName)
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
	pkcs11, err := hwsec.NewPkcs11Util(r, utility)
	if err != nil {
		s.Fatal("PKCS#11 Utility creation error: ", err)
	}

	// Remove all keys/certs, if any at the end. i.e. Cleanup after ourselves.
	defer r.Run(ctx, "sh", "-c", "rm -f "+hwsec.Pkcs11Scratchpad+"/testkey* "+hwsec.Pkcs11Scratchpad+"/testfile*")
	// Note: This test expects a clean /tmp/ (Pkcs11Scratchpad) in the sense that there should be no files prefixed by testkey or testfile when the test start.
	// Note: Also, this test expects a clean keystore, in the sense that there should be no object with the same ID as those used by this test.

	// Create the software-generated, then imported key
	importedKey, err := pkcs11.Pkcs11CreateRsaSoftwareKey(ctx, utility, "", "testkey1", "999999", false, true)
	if err != nil {
		s.Fatal("Failed to create software key: ", err)
	}
	defer func() {
		if err := pkcs11.Pkcs11DestroyKey(ctx, importedKey); err != nil {
			s.Fatal("Failed to clean up software key: ", err)
		}
	}()

	// Create the software-generated, then imported as software-backed key
	softwareKey, err := pkcs11.Pkcs11CreateRsaSoftwareKey(ctx, utility, "", "testkey2", "888888", true, true)
	if err != nil {
		s.Fatal("Failed to create software key: ", err)
	}
	defer func() {
		if err := pkcs11.Pkcs11DestroyKey(ctx, softwareKey); err != nil {
			s.Fatal("Failed to clean up software key: ", err)
		}
	}()

	// Create the TPM generated key
	generatedKey, err := pkcs11.Pkcs11CreateRsaGeneratedKey(ctx, utility, "", "testkey3", "777777")
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
		checkKey(ctx, s, pkcs11, k)
	}
}
