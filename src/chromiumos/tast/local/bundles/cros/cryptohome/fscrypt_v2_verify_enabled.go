// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cryptohome

import (
	"context"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: FscryptV2VerifyEnabled,
		Desc: "Verifies the fscrypt v2 flag is enabled on the platforms where it should be enabled",
		Contacts: []string{
			"dlunev@google.com",
			"chromeos-storage@google.com",
		},
		SoftwareDeps: []string{"fscrypt_v2_support", "no_use_fscrypt_v2"},
		Attr:         []string{"group:mainline", "informational"},
	})
}

func FscryptV2VerifyEnabled(ctx context.Context, s *testing.State) {
	s.Fatal("Fscrypt V2 is not enabled via IUSE, but should be")
}
