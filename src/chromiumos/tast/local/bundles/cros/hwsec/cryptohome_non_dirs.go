// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"os"
	"path/filepath"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/hwsec/util"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CryptohomeNonDirs,
		Desc: "Tests Cryptohome's ability to detect directories with bad dir types in the mount path of a home directory",
		Contacts: []string{
			"cros-hwsec@chromium.org",
			"yich@chromium.org",
		},
		SoftwareDeps: []string{"tpm"},
		Attr:         []string{"group:mainline"},
	})
}

// CryptohomeNonDirs checks that cryptohome could detect directories with bad permissions or ownership in the mount path of a home directory.
func CryptohomeNonDirs(ctx context.Context, s *testing.State) {
	cmdRunner := hwseclocal.NewCmdRunner()
	cryptohome := hwsec.NewCryptohomeClient(cmdRunner)

	const (
		user     = "foo@example.com"
		password = "whatever"
	)

	userPath, err := cryptohome.GetHomeUserPath(ctx, user)
	if err != nil {
		s.Fatal("Failed to get user home path: ", err)
	}
	systemPath, err := cryptohome.GetRootUserPath(ctx, user)
	if err != nil {
		s.Fatal("Failed to get user root path: ", err)
	}

	for _, tc := range []struct {
		name    string
		clobber func(ctx context.Context) error
		cleanup func() error
	}{
		{
			// Leaf element of user path is non-dir.
			name: "leaf_user_path_non_dir",
			clobber: func(ctx context.Context) error {
				file, err := os.Create(userPath)
				if err != nil {
					return errors.Wrap(err, "failed to create user home")
				}
				file.Close()
				return nil
			},
			cleanup: func() error {
				return os.RemoveAll(userPath)
			},
		},
		{
			// Leaf element of system path is non-dir.
			name: "leaf_system_path_non_dir",
			clobber: func(ctx context.Context) error {
				if err := os.Symlink("/etc", systemPath); err != nil {
					return errors.Wrap(err, "failed to create user root")
				}
				return nil
			},
			cleanup: func() error {
				return os.RemoveAll(systemPath)
			},
		},
		{
			// Non-leaf element of user path is non-dir.
			name: "nonleaf_user_path_non_dir",
			clobber: func(ctx context.Context) error {
				parentPath := filepath.Dir(userPath)
				if err := os.Rename(parentPath, parentPath+".old"); err != nil {
					return errors.Wrap(err, "failed to rename parent path")
				}
				file, err := os.Create(parentPath)
				if err != nil {
					return errors.Wrap(err, "failed to create parent of user home")
				}
				file.Close()
				return nil
			},
			cleanup: func() error {
				parentPath := filepath.Dir(userPath)
				if _, err := os.Stat(parentPath + ".old"); !os.IsNotExist(err) {
					if _, err := os.Stat(parentPath); !os.IsNotExist(err) {
						if err := os.Remove(parentPath); err != nil {
							return errors.Wrap(err, "failed to remove parent path")
						}
					}
					return os.Rename(parentPath+".old", parentPath)
				}
				return nil
			},
		},
		{
			// Non-leaf element of system path is non-dir.
			name: "nonleaf_system_path_non_dir",
			clobber: func(ctx context.Context) error {
				parentPath := filepath.Dir(systemPath)
				if err := os.Rename(parentPath, parentPath+".old"); err != nil {
					return errors.Wrap(err, "failed to rename parent path")
				}
				file, err := os.Create(parentPath)
				if err != nil {
					return errors.Wrap(err, "failed to create parent of user root")
				}
				file.Close()
				return nil
			},
			cleanup: func() error {
				parentPath := filepath.Dir(systemPath)
				if _, err := os.Stat(parentPath + ".old"); !os.IsNotExist(err) {
					if _, err := os.Stat(parentPath); !os.IsNotExist(err) {
						if err := os.Remove(parentPath); err != nil {
							return errors.Wrap(err, "failed to remove parent path")
						}
					}
					return os.Rename(parentPath+".old", parentPath)
				}
				return nil
			},
		},
	} {
		s.Run(ctx, tc.name, func(ctx context.Context, s *testing.State) {
			// Make sure the user home and root is clean before running the test.
			if _, err := cryptohome.RemoveVault(ctx, user); err != nil {
				s.Fatal("Failed to cleanup paths: ", err)
			}
			defer func() {
				if err := tc.cleanup(); err != nil {
					s.Error("Failed to cleanup: ", err)
				}
			}()
			if err := tc.clobber(ctx); err != nil {
				s.Fatal("Failed to clobber the data: ", err)
			}
			// The mount vault operation should fail.
			if err := cryptohome.MountVault(ctx, util.PasswordLabel, hwsec.NewPassAuthConfig(user, password), true, hwsec.NewVaultConfig()); err == nil {
				s.Fatal("Mount unexpectedly succeeded")
			}
		})
	}
}
