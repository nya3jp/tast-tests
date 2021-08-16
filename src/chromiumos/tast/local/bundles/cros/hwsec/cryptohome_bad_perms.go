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
		Func: CryptohomeBadPerms,
		Desc: "Tests Cryptohome's ability to detect directories with bad permissions or ownership in the mount path of a home directory",
		Contacts: []string{
			"cros-hwsec@chromium.org",
			"yich@chromium.org",
		},
		SoftwareDeps: []string{"tpm"},
		Attr:         []string{"group:mainline"},
	})
}

func creatDirWithPerms(ctx context.Context, path string, perm os.FileMode, uid, gid int) error {
	if err := os.Mkdir(path, perm); err != nil {
		return errors.Wrapf(err, "failed to create %s", path)
	}
	if err := os.Chown(path, uid, gid); err != nil {
		return errors.Wrapf(err, "failed to chown %s", path)
	}
	return nil
}

// CryptohomeBadPerms checks that cryptohome could detect directories with bad permissions or ownership in the mount path of a home directory.
func CryptohomeBadPerms(ctx context.Context, s *testing.State) {
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

	var restoreInfo os.FileInfo

	for _, tc := range []struct {
		name    string
		clobber func(ctx context.Context) error
		cleanup func() error
	}{
		{
			// Non-leaf element of path not owned by root.
			name: "nonleaf_user_path_not_owned",
			clobber: func(ctx context.Context) error {
				parentPath := filepath.Dir(userPath)
				if err := os.Chown(parentPath, 1, 1); err != nil {
					return errors.Wrap(err, "failed to chown parent path")
				}
				return nil
			},
			cleanup: func() error {
				parentPath := filepath.Dir(userPath)
				return os.Chown(parentPath, 0, 0)
			},
		},
		{
			// Non-leaf element of path too permissive.
			name: "nonleaf_user_path_too_permissive",
			clobber: func(ctx context.Context) error {
				parentPath := filepath.Dir(userPath)
				info, err := os.Stat(parentPath)
				restoreInfo = info
				if err != nil {
					return errors.Wrap(err, "failed to get parent path stat")
				}
				if err := os.Chmod(parentPath, 0777); err != nil {
					return errors.Wrap(err, "failed to chown parent path")
				}
				return nil
			},
			cleanup: func() error {
				parentPath := filepath.Dir(userPath)
				if err := os.Chown(parentPath, 0, 0); err != nil {
					return errors.Wrap(err, "failed to chown back the parent path")
				}
				if restoreInfo != nil {
					return os.Chmod(parentPath, restoreInfo.Mode())
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

			// Reset restoreInfo before clobber, so that additional test cases
			// would not accidentally read the restoreInfo value from previous ones.
			restoreInfo = nil
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
