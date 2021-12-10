// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"time"

	"chromiumos/tast/common/pkcs11"
	"chromiumos/tast/common/pkcs11/pkcs11test"
	"chromiumos/tast/ctxutil"
	hwsecremote "chromiumos/tast/remote/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ChapsRebootStress,
		Desc: "Checks that chaps functions correctly after multiple reboot",
		Contacts: []string{
			"zuan@chromium.org", // Test author
			"cros-hwsec@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"tpm"},
		Timeout:      10 * 60 * time.Minute,
	})
}

const (
	importedKeyID = "AABBCC1234"
)

func ChapsRebootStress(ctx context.Context, s *testing.State) {
	r := hwsecremote.NewCmdRunner(s.DUT())
	helper, err := hwsecremote.NewHelper(r, s.DUT())
	if err != nil {
		s.Fatal("Helper creation error: ", err)
	}
	utility := helper.CryptohomeClient()

	pkcs11Util, err := pkcs11.NewChaps(ctx, r, utility)
	if err != nil {
		s.Fatal("Failed to create PKCS#11 Utility: ", err)
	}

	const scratchpadPath = "/mnt/stateful_partition/ChapsRebootTest"

	// Remove all keys/certs before the test as well.
	if err := pkcs11test.CleanupScratchpad(ctx, r, scratchpadPath); err != nil {
		s.Fatal("Failed to clean scratchpad before the start of test: ", err)
	}
	if err := pkcs11Util.ClearObjectsOfAllType(ctx, 0, importedKeyID); err != nil {
		testing.ContextLogf(ctx, "Failed to remove key ID %q: %q", importedKeyID, err)
	}

	// Prepare the scratchpad.
	f1, f2, err := pkcs11test.PrepareScratchpadAndTestFiles(ctx, r, scratchpadPath)
	if err != nil {
		s.Fatal("Failed to initialize the scratchpad space: ", err)
	}
	// Remove all keys/certs, if any at the end. i.e. Cleanup after ourselves.
	defer pkcs11test.CleanupScratchpad(ctx, r, scratchpadPath)

	importedKey, err := pkcs11Util.CreateRSASoftwareKey(ctx, scratchpadPath, "", "testkey1", importedKeyID, false, true)
	if err != nil {
		s.Fatal("Failed to initialize the testing key: ", err)
	}

	// Give the cleanup 120 seconds to finish.
	shortenedCtx, cancel := ctxutil.Shorten(ctx, 120*time.Second)
	defer cancel()

	if err := pkcs11test.SignAndVerify(shortenedCtx, pkcs11Util, importedKey, f1, f2, &pkcs11.SHA256RSAPKCS); err != nil {
		s.Fatal("Sign and Verify failed: ", err)
	}

	for i := 0; i < 500; i++ {
		if err := helper.Reboot(shortenedCtx); err != nil {
			s.Fatal("Failed to reboot the DUT: ", err)
		}
		s.Logf("Iteration %d", i)
		if err := pkcs11test.SignAndVerify(shortenedCtx, pkcs11Util, importedKey, f1, f2, &pkcs11.SHA256RSAPKCS); err != nil {
			s.Fatal("Sign and Verify failed: ", err)
		}
	}
}
