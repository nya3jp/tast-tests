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
		Func: ChapsPKCS1V15,
		Desc: "Verifies PKCS#1 v1.5 works with RSA keys (sign, verify) in chaps",
		Attr: []string{"group:mainline", "informational"},
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

	pkcs11Util, err := pkcs11.NewChaps(ctx, r, utility)
	if err != nil {
		s.Fatal("Failed to create PKCS#11 Utility: ", err)
	}

	const scratchpadPath = "/tmp/ChapsPKCS1V15Test"

	// Prepare the scratchpad.
	f1, f2, err := pkcs11test.PrepareScratchpadAndTestFiles(ctx, r, scratchpadPath)
	if err != nil {
		s.Fatal("Failed to initialize the scratchpad space: ", err)
	}
	// Remove all keys/certs, if any at the end. i.e. Cleanup after ourselves.
	defer pkcs11test.CleanupScratchpad(ctx, r, scratchpadPath)

	// Create the software-generated, then imported key.
	importedKey, err := pkcs11Util.CreateRSASoftwareKey(ctx, scratchpadPath, "", "testkey1", "aaaaaa")
	if err != nil {
		s.Fatal("Failed to create software key: ", err)
	}
	defer func() {
		if err := pkcs11Util.DestroyKey(ctx, importedKey); err != nil {
			s.Error("Failed to clean up software key: ", err)
		}
	}()

	keys := []*pkcs11.KeyInfo{importedKey}

	// Test the various keys.
	for _, k := range keys {
		// Test the various mechanisms.
		for _, m := range []pkcs11.MechanismInfo{pkcs11.SHA1RSAPKCS, pkcs11.SHA256RSAPKCS} {
			if err := pkcs11test.SignAndVerify(ctx, pkcs11Util, k, f1, f2, &m); err != nil {
				s.Error("SignAndVerify failed: ", err)
			}
		}
	}
}
