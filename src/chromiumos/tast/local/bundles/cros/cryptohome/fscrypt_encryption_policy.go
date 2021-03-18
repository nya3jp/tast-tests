// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cryptohome

import (
	"context"
	"os"
	"path/filepath"

	"github.com/google/fscrypt/metadata"

	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

type fscryptVersionParam struct {
	version int64
}

func init() {
	testing.AddTest(&testing.Test{
		Func: FscryptEncryptionPolicy,
		Desc: "Check fscrypt encryption policy version of a newly created user cryptohome",
		Contacts: []string{
			"sarthakkukreti@google.com",
			"chromeos-storage@google.com",
		},
		Attr: []string{"group:mainline"},
		Params: []testing.Param{{
			Name:              "v1",
			Val:               fscryptVersionParam{version: 1},
			ExtraSoftwareDeps: []string{"use_fscrypt_v1"},
		}, {
			Name:              "v2",
			Val:               fscryptVersionParam{version: 2},
			ExtraSoftwareDeps: []string{"use_fscrypt_v2"},
		}},
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

	if err := cryptohome.CheckService(ctx); err != nil {
		s.Fatal("Cryptohomed not running as expected: ", err)
	}

	// Create user vault.
	if err := cryptohome.CreateVault(ctx, user, password); err != nil {
		s.Fatal("Failed to create user vault: ", err)
	}

	defer func() {
		if err := cryptohome.UnmountVault(ctx, user); err != nil {
			s.Error("Failed to unmount cryptohome vault: ", err)
		}
		if err := cryptohome.RemoveVault(ctx, user); err != nil {
			s.Error("Failed to remove cryptohome vault: ", err)
		}
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

	expectedPolicyVersion := s.Param().(fscryptVersionParam).version

	if encPolicy.Options.PolicyVersion != expectedPolicyVersion {
		s.Fatalf("Invalid policy version: got %d, want %d", encPolicy.Options.PolicyVersion, expectedPolicyVersion)
	}
}
