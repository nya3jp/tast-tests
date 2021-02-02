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
		Attr:         []string{"group:mainline", "informational"},
	})
}

func cleanPath(ctx context.Context, path string) error {
	if _, err := os.Stat(path); os.IsExist(err) {
		if err := os.RemoveAll(path); err != nil {
			errors.Wrapf(err, "failed to cleanup %s before testing", path)
		}
	}
	return nil
}

func requireMountFail(ctx context.Context, utility *hwsec.UtilityCryptohomeBinary, user string) error {
	err := utility.MountVault(ctx, util.FirstUsername, util.FirstPassword, util.PasswordLabel, true, hwsec.NewVaultConfig())
	if err == nil {
		return errors.Errorf("mount unexpectedly succeeded for %s", user)
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

	utility := helper.CryptohomeUtil

	user := util.FirstUsername

	// Make sure the user home and root is clean before running the test.
	userPath, err := utility.GetHomeUserPath(ctx, user)
	if err != nil {
		s.Fatal("Failed to get user home path: ", err)
	}
	if err := cleanPath(ctx, userPath); err != nil {
		s.Fatal("Failed to cleanup user home: ", err)
	}
	systemPath, err := utility.GetRootUserPath(ctx, user)
	if err != nil {
		s.Fatal("Failed to get user root path: ", err)
	}
	if err := cleanPath(ctx, systemPath); err != nil {
		s.Fatal("Failed to cleanup user root: ", err)
	}

	// Leaf element of user path not owned by user.
	func() {
		path := userPath
		if err := os.Mkdir(path, 0755); err != nil {
			s.Fatal("Failed to create user home: ", err)
		}
		defer os.RemoveAll(path)
		if err := os.Chown(path, 0, 0); err != nil {
			s.Fatal("Failed to chwon user home: ", err)
		}
		if err := requireMountFail(ctx, utility, user); err != nil {
			s.Fatal("Unexpected mount succeeded: ", err)
		}
	}()

	// Leaf element of system path not owned by root.
	func() {
		path := systemPath
		if err := os.Mkdir(path, 0755); err != nil {
			s.Fatal("Failed to create user root: ", err)
		}
		defer os.RemoveAll(path)
		if err := os.Chown(path, 1, 1); err != nil {
			s.Fatal("Failed to chwon user root: ", err)
		}
		if err := requireMountFail(ctx, utility, user); err != nil {
			s.Fatal("Unexpected mount succeeded: ", err)
		}
	}()

	// Leaf element of path too permissive.
	func() {
		path := userPath
		if err := os.Mkdir(path, 0777); err != nil {
			s.Fatal("Failed to create user home: ", err)
		}
		defer os.RemoveAll(path)
		if err := os.Chown(path, 1, 1); err != nil {
			s.Fatal("Failed to chwon user home: ", err)
		}
		if err := requireMountFail(ctx, utility, user); err != nil {
			s.Fatal("Unexpected mount succeeded: ", err)
		}
	}()

	// Non-leaf element of path not owned by root.
	func() {
		path := userPath
		parentPath := filepath.Dir(path)
		if err := os.Chown(parentPath, 1, 1); err != nil {
			s.Fatal("Failed to chwon parent path: ", err)
		}
		defer os.Chown(parentPath, 0, 0)
		if err := requireMountFail(ctx, utility, user); err != nil {
			s.Fatal("Unexpected mount succeeded: ", err)
		}
	}()

	// Non-leaf element of path too permissive.
	func() {
		path := userPath
		parentPath := filepath.Dir(path)
		info, err := os.Stat(parentPath)
		if err != nil {
			s.Fatal("Failed to get parent path stat: ", err)
		}
		if err := os.Chown(parentPath, 1, 1); err != nil {
			s.Fatal("Failed to chwon parent path: ", err)
		}
		defer os.Chown(parentPath, 0, 0)
		if err := os.Chmod(parentPath, 0777); err != nil {
			s.Fatal("Failed to chwon parent path: ", err)
		}
		defer os.Chmod(parentPath, info.Mode())
		if err := requireMountFail(ctx, utility, user); err != nil {
			s.Fatal("Unexpected mount succeeded: ", err)
		}
	}()

}
