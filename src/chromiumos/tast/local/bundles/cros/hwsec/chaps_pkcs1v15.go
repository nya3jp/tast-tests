// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"

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

	pkcs11Util := pkcs11.NewUtil(r, utility)

	// Prepare the scratchpad.
	pkcs11Util.PrepareScratchpadAndTestFiles(ctx)
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
			s.Fatal("Failed to clean up software key: ", err)
		}
	}()

	keys := []*pkcs11.KeyInfo{importedKey}

	// Test the various keys.
	for _, k := range keys {
		// Test the various mechanisms.
		for _, m := range []pkcs11.MechanismInfo{pkcs11Util.MechanismSha1RsaPkcs(), pkcs11Util.MechanismSha256RsaPkcs()} {
			if err := k.SignVerify(ctx, pkcs11.TestFile1Path, pkcs11.TestFile2Path, m); err != nil {
				s.Fatal("SignVerify failed: ", err)
			}
		}
	}
}
