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
		Func: CryptohomeNonDirs,
		Desc: "Tests Cryptohome's ability to detect directories with bad dir types in the mount path of a home directory",
		Contacts: []string{
			"cros-hwsec@chromium.org",
			"yich@chromium.org",
		},
		SoftwareDeps: []string{"tpm"},
		Attr:         []string{"group:mainline", "informational"},
	})
}

// CryptohomeNonDirs checks that cryptohome could detect directories with bad permissions or ownership in the mount path of a home directory.
func CryptohomeNonDirs(ctx context.Context, s *testing.State) {
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

	for _, tc := range []struct {
		task        string
		clobberFunc func(ctx context.Context) error
		cleanupFunc func()
	}{
		{
			// Leaf element of user path is non-dir.
			task: "leaf_user_path_non_dir",
			clobberFunc: func(ctx context.Context) error {
				file, err := os.Create(userPath)
				if err != nil {
					return errors.Wrap(err, "failed to create user home")
				}
				file.Close()
				return nil
			},
			cleanupFunc: func() {
				os.RemoveAll(userPath)
			},
		},
		{
			// Leaf element of system path is non-dir.
			task: "leaf_system_path_non_dir",
			clobberFunc: func(ctx context.Context) error {
				if err := os.Symlink("/etc", systemPath); err != nil {
					return errors.Wrap(err, "failed to create user root")
				}
				return nil
			},
			cleanupFunc: func() {
				os.RemoveAll(systemPath)
			},
		},
		{
			// Non-leaf element of user path is non-dir.
			task: "nonleaf_user_path_non_dir",
			clobberFunc: func(ctx context.Context) error {
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
			cleanupFunc: func() {
				parentPath := filepath.Dir(userPath)
				if _, err := os.Stat(parentPath + ".old"); os.IsExist(err) {
					if _, err := os.Stat(parentPath); os.IsExist(err) {
						os.Remove(parentPath)
					}
					os.Rename(parentPath+".old", parentPath)
				}
			},
		},
		{
			// Non-leaf element of system path is non-dir.
			task: "nonleaf_system_path_non_dir",
			clobberFunc: func(ctx context.Context) error {
				parentPath := filepath.Dir(systemPath)
				if err := os.Rename(parentPath, parentPath+".old"); err != nil {
					return errors.Wrap(err, "failed to rename parent path")
				}
				defer func() {
				}()
				file, err := os.Create(parentPath)
				if err != nil {
					return errors.Wrap(err, "failed to create parent of user root")
				}
				file.Close()
				return nil
			},
			cleanupFunc: func() {
				parentPath := filepath.Dir(systemPath)
				if _, err := os.Stat(parentPath + ".old"); os.IsExist(err) {
					if _, err := os.Stat(parentPath); os.IsExist(err) {
						os.Remove(parentPath)
					}
					os.Rename(parentPath+".old", parentPath)
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
