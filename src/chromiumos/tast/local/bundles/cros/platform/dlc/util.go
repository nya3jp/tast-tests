// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package dlc provides functionality used by several DLC tests
// but not necessary for tests that simply use DLC.
package dlc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"chromiumos/tast/common/testexec"
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

// ListOutput holds the output from running `dlcservice_util --list`.
type ListOutput struct {
	ID        string `json:"id"`
	Package   string `json:"package"`
	RootMount string `json:"root_mount"`
}

// removeExt removes the suffix extension for the given path.
func removeExt(path string) string {
	return strings.TrimSuffix(path, filepath.Ext(path))
}

// getRootMounts retrieves the root mounts for the given id.
func getRootMounts(path, id string) ([]string, error) {
	rootMounts := make([]string, 0)
	m, err := dlcList(path)
	if err != nil {
		return nil, err
	}
	if l, ok := m[id]; ok {
		for _, val := range l {
			rootMounts = append(rootMounts, val.RootMount)
		}
	}
	return rootMounts, nil
}

// checkSHA2Sum checks the SHA256 sum by reading in the expected SHA256 sum
// from the given hashPath, then matches it against corresponding file.
func checkSHA2Sum(hashPath string) error {
	// Get the actual SHA256 sum.
	path := removeExt(hashPath)
	actualBytes, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	actualSumBytes := sha256.Sum256(actualBytes)
	actualSum := hex.EncodeToString(actualSumBytes[:])

	// Get the expected SHA256 sum.
	expectedBytes, err := ioutil.ReadFile(hashPath)
	if err != nil {
		return err
	}
	// hashPath contains a file with format as "<sha256sum> <filename>"
	expectedSum := strings.Fields(string(expectedBytes))[0]

	if actualSum != expectedSum {
		return errors.Errorf("mismatch in SHA256 checksum for %s. Actual=%s Expected=%s", path, actualSum, expectedSum)
	}
	return nil
}

// checkPerms checks the file permissions by reading in the expected
// permissions from the given parmsPath, then matches it against the
// corresponding file.
func checkPerms(parmsPath string) error {
	// Get the actual permisisons.
	path := removeExt(parmsPath)
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	actualPerm := fmt.Sprintf("%#o", info.Mode().Perm())

	// Get the expected permissions.
	permsBytes, err := ioutil.ReadFile(parmsPath)
	if err != nil {
		return err
	}
	expectedPerm := strings.TrimSpace(string(permsBytes))

	if actualPerm != expectedPerm {
		return errors.Errorf("mismatch in permissions for %s. Actual=%s Expected=%s", path, actualPerm, expectedPerm)
	}
	return nil
}

// verifyDlcContent verifies that the contents of the DLC have valid file
// hashes and file permissions.
func verifyDlcContent(path, id string) error {
	rootMounts, err := getRootMounts(path, id)
	if err != nil {
		return err
	}
	if len(rootMounts) == 0 {
		return errors.Errorf("no root mounts exist for %v", id)
	}
	for _, rootMount := range rootMounts {
		if err := filepath.Walk(rootMount, func(path string, info os.FileInfo, err error) error {
			switch filepath.Ext(path) {
			case ".sum":
				if err := checkSHA2Sum(path); err != nil {
					return errors.Wrap(err, "check sum failed")
				}
				break
			case ".perms":
				if err := checkPerms(path); err != nil {
					return errors.Wrap(err, "permissions check failed")
				}
				break
			}
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

// dlcList reads in the given path and then converts it to a map of structs.
func dlcList(path string) (map[string][]ListOutput, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	listOutput := make(map[string][]ListOutput)
	if err := json.Unmarshal(b, &listOutput); err != nil {
		return nil, err
	}
	return listOutput, nil
}

// listDlcs is a helper to call into dlcservice_util for `--list` option and
// will dump the output at the given path in json format. If the path already
// exists, it will return an error.
func listDlcs(ctx context.Context, path string) error {
	// Path already exists.
	if _, err := os.Stat(path); err == nil {
		return errors.Wrapf(err, "file already exists at: %v", path)
	}
	cmd := testexec.CommandContext(ctx, "dlcservice_util", "--list", "--dump="+path)
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to list installed DLCs")
	}
	// TODO(kimjae): Fix dlcservice_util to throw error when dumping fails.
	// Until then keep this check.
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return errors.New("dlcservice_util failed to dump")
	}
	return nil
}

// GetInstalled uses the dlcservice_util to get the installed DLCs.
// Will return an error when failing to retrieve and parse the output.
func GetInstalled(ctx context.Context) ([]ListOutput, error) {
	testing.ContextLog(ctx, "Getting installed DLCs")
	d, ok := testing.ContextOutDir(ctx)
	if !ok {
		return nil, errors.New("failed to get OutDir from context")
	}
	path := filepath.Join(d, "tmp_get_installed")
	defer os.Remove(path)
	// listDlcs needs a non-existent file.
	if err := listDlcs(ctx, path); err != nil {
		return nil, err
	}
	m, err := dlcList(path)
	if err != nil {
		return nil, err
	}
	var installedIDs []ListOutput
	for id, l := range m {
		for _, val := range l {
			if id != val.ID {
				return nil, errors.Errorf("list has mismatching IDs: %s %s", id, val.ID)
			}
			if val.Package == "" {
				return nil, errors.Errorf("empty package for ID: %s", id)
			}
			installedIDs = append(installedIDs, val)
		}
	}
	return installedIDs, nil
}

// DumpAndVerifyInstalledDLCs calls dlcservice's GetInstalled D-Bus method
// via dlcservice_util command.
func DumpAndVerifyInstalledDLCs(ctx context.Context, dumpPath, tag string, ids ...string) error {
	testing.ContextLog(ctx, "Asking dlcservice for installed DLC modules")
	f := tag + ".log"
	path := filepath.Join(dumpPath, f)
	if err := listDlcs(ctx, path); err != nil {
		return err
	}
	for _, id := range ids {
		if err := verifyDlcContent(path, id); err != nil {
			return err
		}
	}
	return nil
}

// CopyFileWithPermissions copies file |from| to |to| and sets permissions.
func CopyFileWithPermissions(from, to string, perms os.FileMode) error {
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
