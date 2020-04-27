// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"time"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/common/pkcs11"
	"chromiumos/tast/common/pkcs11/pkcs11test"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/hwsec/util"
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

	// Remove all keys/certs before the test as well.
	if err := pkcs11test.CleanupScratchpad(ctx, r, scratchpadPath); err != nil {
		s.Fatal("Failed to clean scratchpad before the start of test: ", err)
	}
	util.CleanupKeysBeforeTest(ctx, pkcs11Util, utility)

	// Prepare the scratchpad.
	f1, f2, err := pkcs11test.PrepareScratchpadAndTestFiles(ctx, r, scratchpadPath)
	if err != nil {
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
		// Test the various mechanisms.
		for _, m := range []pkcs11.MechanismInfo{pkcs11.SHA1RSAPKCS, pkcs11.SHA256RSAPKCS} {
			if err := pkcs11test.SignAndVerify(shortenedCtx, pkcs11Util, k, f1, f2, &m); err != nil {
				s.Error("SignAndVerify failed: ", err)
			}
		}
	}
}
