// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"regexp"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ECDHShortKey,
		Desc: "Verifies a short ECC key (leading byte(s) of X/Y coordinate being 0) works correctly",
		Contacts: []string{
			"cylai@google.com", // Test author.
			"cros-hwsec@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"tpm2"},
	})
}

// ECDHShortKey verifies ECDD can work with short ECC keys.
func ECDHShortKey(ctx context.Context, s *testing.State) {

	const filename = "/tmp/keyblob"

	// Create a key for testing.
	if _, err := testexec.CommandContext(ctx, "trunks_client", "--key_create", "--ecc", "--usage=all", "--key_blob="+filename).Output(); err != nil {
		s.Fatal("Failed to create an ECC key: ", err)
	}

	// Load the key back to TPM.
	out, err := testexec.CommandContext(ctx, "trunks_client", "--key_load", "--key_blob="+filename).Output()
	if err != nil {
		s.Fatal("Failed to load the key from key blob: ", err)
	}
	// Parse the handle from the command output. The example output is "Loaded key handle: 0x80000003".
	re := regexp.MustCompile(`0[xX][0-9a-fA-F]+`)
	matches := re.FindAllString(string(out), -1)
	if matches == nil || len(matches) != 1 {
		s.Fatal("Failed to parse loaded key handle: ", out)
	}

	handle := matches[0]

	defer func() {
		if _, err := testexec.CommandContext(ctx, "trunks_client", "--key_unload", "--handle="+handle).Output(); err != nil {
			s.Errorf("Failed to unload key handle 0x%x with error: %v",
				handle, err)
		}
	}()

	// Test if the TPM key can work with a short remote ECC key.
	if out, err := testexec.CommandContext(ctx, "trunks_client", "--key_test_short_ecc", "--handle="+handle).Output(); err != nil {
		s.Error("Failed to perform ECDH with a short key: ", err)
		// All the output is valueable for debugging; Hopefully it is short enough (around 4 lines).
		s.Fatal(out)
	}
}
