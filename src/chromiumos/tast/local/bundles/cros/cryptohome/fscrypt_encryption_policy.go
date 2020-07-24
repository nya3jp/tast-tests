// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cryptohome

import (
	"context"
	"os"
	"path/filepath"

	"github.com/google/fscrypt/metadata"
	"github.com/google/fscrypt/util"
	"golang.org/x/sys/unix"

	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: FscryptEncryptionPolicy,
		Desc: "Check fscrypt encryption policy version of a newly created user cryptohome",
		Contacts: []string{
			"sarthakkukreti@google.com",
			"chromeos-storage@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
	})
}

func FscryptEncryptionPolicy(ctx context.Context, s *testing.State) {
	const (
		shadow   = "/home/.shadow"
		user     = "fscryptuser"
		password = "pass"
	)

	// Make sure cryptohomed is running.
	if err := upstart.EnsureJobRunning(ctx, "cryptohomed"); err != nil {
		s.Fatal("Failed to start cryptohomed: ", err)
	}

	// Create user vault.
	if err := cryptohome.CreateVault(ctx, user, password); err != nil {
		s.Fatal("Failed to create user vault: ", err)
	}

	defer func() {
		cryptohome.UnmountVault(ctx, user)
		cryptohome.RemoveVault(ctx, user)
	}()

	hash, err := cryptohome.UserHash(ctx, user)
	if err != nil {
		s.Fatal("Failed to get user hash: ", err)
	}

	mountPath := filepath.Join(shadow, hash, "mount")
	if _, err := os.Stat(mountPath); err != nil {
		s.Fatal("Mount path not found: ", err)
	}

	// Get policy for the currently mounted user.
	encPolicy, err := metadata.GetPolicy(mountPath)
	if err != nil {
		s.Fatal("Failed to get policy for mount path: ", err)
	}

	var expectedPolicyVersion int64 = unix.FSCRYPT_POLICY_V1
	if util.IsKernelVersionAtLeast(5, 4) {
		expectedPolicyVersion = unix.FSCRYPT_POLICY_V2
	}

	if encPolicy.Options.PolicyVersion != expectedPolicyVersion {
		s.Fatalf("Invalid policy version: expected %d actual %d", encPolicy.Options.PolicyVersion, expectedPolicyVersion)
	}
}
