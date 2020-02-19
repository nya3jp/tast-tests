// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cryptohome

import (
	"context"

	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: GuestSession,
		Desc: "Ensures that cryptohome correctly mounts guest sessions",
		Contacts: []string{
			"jorgelo@chromium.org",
			"chromeos-security@google.com",
		},
		Attr: []string{"group:mainline"},
	})
}

func GuestSession(ctx context.Context, s *testing.State) {
	if err := cryptohome.MountGuest(ctx); err != nil {
		s.Fatal("Failed to mount guest: ", err)
	}
	defer cryptohome.UnmountVault(ctx, cryptohome.GuestUser)
}
