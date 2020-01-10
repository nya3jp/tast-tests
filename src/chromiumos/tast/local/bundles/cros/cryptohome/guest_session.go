// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cryptohome

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/testexec"
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

	// Use a shortened context for subprocesses.
	shortCtx, shortCancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer shortCancel()

	// Get list of mounts.
	cmd := testexec.CommandContext(shortCtx, "mount")
	output, err := cmd.Output()
	if err != nil {
		s.Fatal("Failed to get list of mounts: ", err)
	}

	// Check that an ephemeral mount exists.
	mounts := string(output)
	const ephemeralMountPoint string = "/run/cryptohome/ephemeral_mount"
	if !strings.Contains(mounts, ephemeralMountPoint) {
		s.Fatalf("Could not find %q in mounts", ephemeralMountPoint)
	}
}
