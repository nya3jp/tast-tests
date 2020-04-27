// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/common/pkcs11"
	"chromiumos/tast/common/pkcs11/pkcs11test"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/hwsec/util"
	libhwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ChapsAttributePolicy,
		Desc: "Verifies Chaps Attribute policy works as intended",
		Attr: []string{"group:mainline", "informational"},
		Contacts: []string{
			"zuan@chromium.org",
			"cros-hwsec@chromium.org",
		},
		SoftwareDeps: []string{"chrome"},
	})
}

func checkCannotRead(ctx context.Context, s *testing.State, pkcs11Util *pkcs11.Chaps, k *pkcs11.KeyInfo, attributeName string) {
	res, err := pkcs11Util.GetObjectAttribute(ctx, k, "privkey", attributeName)
	if err == nil {
		s.Fatalf("%q readable when it shouldn't be, got %q", attributeName, res)
	}
	var perr *pkcs11.Error
	if !errors.As(err, &perr) {
		s.Error("Error from GetObjectAttribute() is not a PKCS#11 error: ", err)
	} else {
		if perr.PKCS11RetCode == "CKR_ATTRIBUTE_TYPE_INVALID" {
			s.Log(attributeName + " doesn't exist.")
		} else if perr.PKCS11RetCode == "CKR_ATTRIBUTE_SENSITIVE" {
			s.Log(attributeName + " is unreadable (as it should be).")
		} else {
			s.Errorf("Incorrect error code %q when testing if %q is readable", perr.PKCS11RetCode, attributeName)
		}
	}
}

func checkCannotWrite(ctx context.Context, s *testing.State, pkcs11Util *pkcs11.Chaps, k *pkcs11.KeyInfo, attributeName string) {
	err := pkcs11Util.SetObjectAttribute(ctx, k, "privkey", attributeName, "01")
	if err == nil {
		s.Fatalf("%q writable when it shouldn't be", attributeName)
	}
	var perr *pkcs11.Error
	if !errors.As(err, &perr) {
		s.Error("Error from SetObjectAttribute() is not a PKCS#11 error: ", err)
	} else {
		if perr.PKCS11RetCode != "CKR_ATTRIBUTE_READ_ONLY" {
			s.Errorf("Incorrect error code %q when testing if %q is writable", err.Error(), attributeName)
		}
	}
}

func checkCannotWriteOnCopy(ctx context.Context, s *testing.State, pkcs11Util *pkcs11.Chaps, k *pkcs11.KeyInfo, attributeName string) {
	attributeMap := map[string]string{}
	attributeMap[attributeName] = "01"
	newKey, msg, err := pkcs11Util.CreateKeyCopy(ctx, k, "BAADF00D", attributeMap)
	if err == nil {
		// Destroy the key that we've accidentally created.
		pkcs11Util.DestroyKey(ctx, newKey)

		// Fail the test because such key should not be created.
		s.Errorf("%q is settable on copy", attributeName)
	}

	if !strings.Contains(msg, "CKR_ATTRIBUTE_READ_ONLY") {
		s.Errorf("Incorrect error message %q when testing if %q is writable on copy", msg, attributeName)
	}
}

func checkKey(ctx context.Context, s *testing.State, pkcs11Util *pkcs11.Chaps, k *pkcs11.KeyInfo) {
	const IssuerTestValue = "AABBCC"

	// Sanity test that whatever that should be writable and readable should be so. CKA_ISSUER is used here.
	if err := pkcs11Util.SetObjectAttribute(ctx, k, "privkey", "CKA_ISSUER", IssuerTestValue); err != nil {
		s.Fatal("Unable to set CKA_ISSUER attribute: ", err)
	}

	// Read it back to check that it's set correctly.
	res, err := pkcs11Util.GetObjectAttribute(ctx, k, "privkey", "CKA_ISSUER")
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
	pkcs11Util, err := pkcs11.NewChaps(ctx, r, utility)
	if err != nil {
		s.Fatal("Failed to create PKCS#11 Utility: ", err)
	}

	const scratchpadPath = "/tmp/ChapsAttributePolicyTest"

	// Remove all keys/certs before the test as well.
	if err := pkcs11test.CleanupScratchpad(ctx, r, scratchpadPath); err != nil {
		s.Fatal("Failed to clean scratchpad before the start of test: ", err)
	}
	util.CleanupKeysBeforeTest(ctx, pkcs11Util, utility)

	// Prepare the scratchpad.
	if _, _, err := pkcs11test.PrepareScratchpadAndTestFiles(ctx, r, scratchpadPath); err != nil {
		s.Fatal("Failed to initialize the scratchpad space: ", err)
	}
	// Remove all keys/certs, if any at the end. i.e. Cleanup after ourselves.
	defer pkcs11test.CleanupScratchpad(ctx, r, scratchpadPath)

	// Create the various keys.
	keys, err := util.CreateKeysForTesting(ctx, r, pkcs11Util, utility, scratchpadPath)
	if err != nil {
		s.Fatal("Failed to create keys for testing: ", err)
	}
	defer func() {
		if err := util.CleanupTestingKeys(ctx, keys, pkcs11Util, utility); err != nil {
			s.Error("Failed to cleanup testing keys: ", err)
		}
	}()
	// Give the cleanup 10 seconds to finish.
	shortenedCtx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Test the various keys.
	for _, k := range keys {
		checkKey(shortenedCtx, s, pkcs11Util, k)
	}
}
