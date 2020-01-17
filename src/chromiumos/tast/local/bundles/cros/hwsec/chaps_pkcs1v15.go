// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"fmt"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/common/pkcs11"
	libhwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ChapsPKCS1V15,
		Desc: "Verifies PKCS#1 v1.5 works with RSA keys (sign, verify) in chaps",
		Attr: []string{"informational"},
		Contacts: []string{
			"zuan@chromium.org",
			"cros-hwsec@chromium.org",
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

	pkcs11Util, err := pkcs11.NewUtil(ctx, r, utility)
	if err != nil {
		s.Fatal("Failed to create PKCS#11 Utility: ", err)
	}

	// Prepare the scratchpad.
	testFile1Path, testFile2Path, err := pkcs11Util.PrepareScratchpadAndTestFiles(ctx, "/tmp/ChapsPKCS1V15Test")
	if err != nil {
		s.Fatal("Failed to initialize the scratchpad space: ", err)
	}
	// Remove all keys/certs, if any at the end. i.e. Cleanup after ourselves.
	defer pkcs11Util.CleanupScratchpad(ctx)
	// Note: Also, this test expects a clean keystore, in the sense that there should be no object with the same ID as those used by this test.

	// Create the software-generated, then imported key.
	importedKey, err := pkcs11Util.CreateRsaSoftwareKey(ctx, utility, "", "testkey1", "aaaaaa")
	if err != nil {
		s.Fatal("Failed to create software key: ", err)
	}
	defer func() {
		if err := pkcs11Util.DestroyKey(ctx, importedKey); err != nil {
			s.Error("Failed to clean up software key: ", err)
		}
	}()

	// Create the TPM generated key.
	generatedKey, err := pkcs11Util.CreateRsaGeneratedKey(ctx, utility, "", "testkey3", "cccccc")
	if err != nil {
		s.Fatal("Failed to create generated key: ", err)
	}
	defer func() {
		if err := pkcs11Util.DestroyKey(ctx, generatedKey); err != nil {
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
			if err := pkcs11Util.DestroyKey(ctx, k); err != nil {
				s.Error("Failed to clean up copied key: ", err)
			}
		}
	}()

	keys = append(keys, copiedKeys...)

	// Test the various keys.
	for _, k := range keys {
		// Test the various mechanisms.
		for _, m := range []pkcs11.MechanismInfo{pkcs11.GetMechanism("SHA1-RSA-PKCS"), pkcs11.GetMechanism("SHA256-RSA-PKCS")} {
			if err := k.SignVerify(ctx, pkcs11Util, testFile1Path, testFile2Path, m); err != nil {
				s.Fatal("SignVerify failed: ", err)
			}
		}
	}
}
