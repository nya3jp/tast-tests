// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"os"
	"path/filepath"

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

	// Make sure the user home and root is clean before running the test.
	if err := helper.CleanupUserPaths(ctx, user); err != nil {
		s.Fatal("Failed to cleanup paths: ", err)
	}

	// Leaf element of user path is non-dir.
	func() {
		path := userPath
		file, err := os.Create(path)
		if err != nil {
			s.Fatal("Failed to create user home: ", err)
		}
		file.Close()
		defer os.RemoveAll(path)
		if err := util.RequireMountFail(ctx, utility, user, pass, label); err != nil {
			s.Fatal("Unexpected mount succeeded: ", err)
		}
	}()

	// Make sure the user home and root is clean before running the test.
	if err := helper.CleanupUserPaths(ctx, user); err != nil {
		s.Fatal("Failed to cleanup paths: ", err)
	}

	// Leaf element of system path is non-dir.
	func() {
		path := systemPath
		if err := os.Symlink("/etc", path); err != nil {
			s.Fatal("Failed to create user root: ", err)
		}
		defer os.RemoveAll(path)
		if err := util.RequireMountFail(ctx, utility, user, pass, label); err != nil {
			s.Fatal("Unexpected mount succeeded: ", err)
		}
	}()

	// Make sure the user home and root is clean before running the test.
	if err := helper.CleanupUserPaths(ctx, user); err != nil {
		s.Fatal("Failed to cleanup paths: ", err)
	}

	// Non-leaf element of user path is non-dir.
	func() {
		path := userPath
		parentPath := filepath.Dir(path)
		if err := os.Rename(parentPath, parentPath+".old"); err != nil {
			s.Fatal("Failed to rename parent path: ", err)
		}
		defer func() {
			if _, err := os.Stat(parentPath); os.IsExist(err) {
				os.Remove(parentPath)
			}
			os.Rename(parentPath+".old", parentPath)
		}()
		file, err := os.Create(parentPath)
		if err != nil {
			s.Fatal("Failed to create parent of user home: ", err)
		}
		file.Close()
		if err := util.RequireMountFail(ctx, utility, user, pass, label); err != nil {
			s.Fatal("Unexpected mount succeeded: ", err)
		}
	}()

	// Make sure the user home and root is clean before running the test.
	if err := helper.CleanupUserPaths(ctx, user); err != nil {
		s.Fatal("Failed to cleanup paths: ", err)
	}

	// Non-leaf element of system path is non-dir.
	func() {
		path := systemPath
		parentPath := filepath.Dir(path)
		if err := os.Rename(parentPath, parentPath+".old"); err != nil {
			s.Fatal("Failed to rename parent path: ", err)
		}
		defer func() {
			if _, err := os.Stat(parentPath); os.IsExist(err) {
				os.Remove(parentPath)
			}
			os.Rename(parentPath+".old", parentPath)
		}()
		file, err := os.Create(parentPath)
		if err != nil {
			s.Fatal("Failed to create parent of user root: ", err)
		}
		file.Close()
		if err := util.RequireMountFail(ctx, utility, user, pass, label); err != nil {
			s.Fatal("Unexpected mount succeeded: ", err)
		}
	}()
}
