// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"os"
	"path/filepath"

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
		Attr:         []string{"group:mainline", "informational"},
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
	cmdRunner, err := hwseclocal.NewCmdRunner()
	if err != nil {
		s.Fatal("Failed to create CmdRunner: ", err)
	}
	helper, err := hwseclocal.NewHelper(cmdRunner)
	if err != nil {
		s.Fatal("Failed to create hwsec local helper: ", err)
	}

	utility := helper.CryptohomeUtil()

	user := util.FirstUsername
	pass := util.FirstPassword
	label := util.PasswordLabel

	userPath, err := utility.GetHomeUserPath(ctx, user)
	if err != nil {
		s.Fatal("Failed to get user home path: ", err)
	}
	systemPath, err := utility.GetRootUserPath(ctx, user)
	if err != nil {
		s.Fatal("Failed to get user root path: ", err)
	}

	var restoreInfo os.FileInfo

	for _, tc := range []struct {
		task        string
		clobberFunc func(ctx context.Context) error
		cleanupFunc func()
	}{
		{
			// Leaf element of user path not owned by user.
			task: "leaf_user_path_not_owned",
			clobberFunc: func(ctx context.Context) error {
				if err := creatDirWithPerms(ctx, userPath, 0755, 0, 0); err != nil {
					return errors.Wrap(err, "failed to create user home")
				}
				return nil
			},
			cleanupFunc: func() {
				os.RemoveAll(userPath)
			},
		},
		{
			// Leaf element of system path not owned by root.
			task: "leaf_system_path_not_owned",
			clobberFunc: func(ctx context.Context) error {
				if err := creatDirWithPerms(ctx, systemPath, 0755, 1, 1); err != nil {
					return errors.Wrap(err, "failed to create user root")
				}
				return nil
			},
			cleanupFunc: func() {
				os.RemoveAll(systemPath)
			},
		},
		{
			// Leaf element of path too permissive.
			task: "leaf_user_path_too_permissive",
			clobberFunc: func(ctx context.Context) error {
				if err := creatDirWithPerms(ctx, userPath, 0777, 1, 1); err != nil {
					return errors.Wrap(err, "failed to create user home")
				}
				return nil
			},
			cleanupFunc: func() {
				os.RemoveAll(userPath)
			},
		},
		{
			// Non-leaf element of path not owned by root.
			task: "nonleaf_user_path_not_owned",
			clobberFunc: func(ctx context.Context) error {
				parentPath := filepath.Dir(userPath)
				if err := os.Chown(parentPath, 1, 1); err != nil {
					return errors.Wrap(err, "failed to chown parent path")
				}
				return nil
			},
			cleanupFunc: func() {
				parentPath := filepath.Dir(userPath)
				os.Chown(parentPath, 0, 0)
			},
		},
		{
			// Non-leaf element of path too permissive.
			task: "nonleaf_user_path_too_permissive",
			clobberFunc: func(ctx context.Context) error {
				parentPath := filepath.Dir(userPath)
				info, err := os.Stat(parentPath)
				restoreInfo = info
				if err != nil {
					return errors.Wrap(err, "failed to get parent path stat")
				}
				if err := os.Chown(parentPath, 1, 1); err != nil {
					return errors.Wrap(err, "failed to chown parent path")
				}
				if err := os.Chmod(parentPath, 0777); err != nil {
					return errors.Wrap(err, "failed to chown parent path")
				}
				return nil
			},
			cleanupFunc: func() {
				parentPath := filepath.Dir(userPath)
				os.Chown(parentPath, 0, 0)
				if restoreInfo != nil {
					os.Chmod(parentPath, restoreInfo.Mode())
				}
			},
		},
	} {
		s.Run(ctx, tc.task, func(ctx context.Context, s *testing.State) {
			// Make sure the user home and root is clean before running the test.
			if err := helper.CleanupUserPaths(ctx, user); err != nil {
				s.Fatal("Failed to cleanup paths: ", err)
			}
			defer tc.cleanupFunc()
			if err := tc.clobberFunc(ctx); err != nil {
				s.Fatal("Failed to clobber the data: ", err)
			}
			if err := util.RequireMountFail(ctx, utility, user, pass, label); err != nil {
				s.Fatal("Unexpected mount succeeded: ", err)
			}
		})
	}
}
