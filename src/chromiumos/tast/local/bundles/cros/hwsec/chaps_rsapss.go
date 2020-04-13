// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/common/pkcs11"
	"chromiumos/tast/common/pkcs11/pkcs11test"
	libhwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ChapsRSAPSS,
		Desc: "Verifies RSA PSS works with RSA keys (sign, verify, encrypt, decrypt) in chaps",
		Attr: []string{"group:mainline", "informational"},
		Contacts: []string{
			"zuan@chromium.org",
			"cros-hwsec@chromium.org",
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
	pkcs11Util, err := pkcs11.NewChaps(ctx, r, utility)
	if err != nil {
		s.Fatal("Failed to create PKCS#11 Utility: ", err)
	}

	const scratchpadPath = "/tmp/ChapsRSAPSSTest"

	// Prepare the scratchpad.
	f1, f2, err := pkcs11test.PrepareScratchpadAndTestFiles(ctx, r, scratchpadPath)
	if err != nil {
		s.Fatal("Failed to initialize the scratchpad space: ", err)
	}
	// Remove all keys/certs, if any at the end. i.e. Cleanup after ourselves.
	defer pkcs11test.CleanupScratchpad(ctx, r, scratchpadPath)

	// Create the software-generated, then imported key.
	importedKey, err := pkcs11Util.CreateRSASoftwareKey(ctx, scratchpadPath, "", "testkey1", "111111")
	if err != nil {
		s.Fatal("Failed to create software key: ", err)
	}
	defer func() {
		if err := pkcs11Util.DestroyKey(ctx, importedKey); err != nil {
			s.Error("Failed to clean up software key: ", err)
		}
	}()

	// Create the TPM generated key.
	generatedKey, err := pkcs11Util.CreateRsaGeneratedKey(ctx, scratchpadPath, "", "testkey3", "333333")
	if err != nil {
		s.Fatal("Failed to create generated key: ", err)
	}
	defer func() {
		if err := pkcs11Util.DestroyKey(ctx, generatedKey); err != nil {
			s.Error("Failed to clean up generated key: ", err)
		}
	}()

	keys := []*pkcs11.KeyInfo{importedKey, generatedKey}

	// Test the various keys.
	for _, k := range keys {
		// Test the various mechanisms.
		for _, m := range []pkcs11.MechanismInfo{pkcs11.SHA1RSAPKCSPSS, pkcs11.SHA256RSAPKCSPSS, pkcs11.GenericRSAPKCSPSSWithSHA1, pkcs11.GenericRSAPKCSPSSWithSHA256} {
			if err = pkcs11test.SignAndVerify(ctx, pkcs11Util, k, f1, f2, &m); err != nil {
				s.Error("SignAndVerify failed: ", err)
			}
		}
	}
}
