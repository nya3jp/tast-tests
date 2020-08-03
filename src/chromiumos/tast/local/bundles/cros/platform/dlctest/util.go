// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package dlctest provides functionality used by several DLC tests
// but not necessary for tests that simply use DLC.
package dlctest

import (
	"context"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strconv"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/dlc"
	"chromiumos/tast/testing"
)

// Constants used by DLC tests
const (
	DirPerm     = 0755
	FilePerm    = 0644
	ImageFile   = "dlc.img"
	SlotA       = "dlc_a"
	SlotB       = "dlc_b"
	TestDir     = "/usr/local/dlc"
	TestID1     = "test1-dlc"
	TestID2     = "test2-dlc"
	TestPackage = "test-package"
)

// DumpAndVerifyInstalledDLCs calls dlcservice's GetInstalled D-Bus method
// via dlcservice_util command.
func DumpAndVerifyInstalledDLCs(ctx context.Context, dumpPath, tag string, ids ...string) error {
	testing.ContextLog(ctx, "Asking dlcservice for installed DLC modules")
	f := tag + ".log"
	path := filepath.Join(dumpPath, f)
	if err := dlc.ListDlcs(ctx, path); err != nil {
		return err
	}
	for _, id := range ids {
		if err := dlc.VerifyDlcContent(path, id); err != nil {
			return err
		}
	}
	return nil
}

// CopyFileAndChangePermissions copies file |from| to |to| and sets permissions.
func CopyFileAndChangePermissions(from, to string, perms os.FileMode) error {
	b, err := ioutil.ReadFile(from)
	if err != nil {
		return errors.Wrap(err, "failed to read file")
	}
	if err := ioutil.WriteFile(to, b, perms); err != nil {
		return errors.Wrap(err, "failed to write file")
	}
	return nil
}

// ChownContentsToDlcservice recursively changes the ownership of the directory
// contents to the uid and gid of dlcservice.
func ChownContentsToDlcservice(dir string) error {
	var u *user.User
	var err error
	if u, err = user.Lookup(dlc.User); err != nil {
		return err
	}

	var uid, gid int64
	if uid, err = strconv.ParseInt(u.Uid, 10, 32); err != nil {
		return errors.Wrapf(err, "failed to parse uid %q", u.Uid)
	}
	if gid, err = strconv.ParseInt(u.Gid, 10, 32); err != nil {
		return errors.Wrapf(err, "failed to parse gid %q", u.Gid)
	}

	return filepath.Walk(dir, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return os.Chown(p, int(uid), int(gid))
	})
}
