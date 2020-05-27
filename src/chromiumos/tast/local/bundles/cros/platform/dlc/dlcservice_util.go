// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package dlc provides ways to interact with dlcservice daemon and utilities.
package dlc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
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

// listDlcs is a helper to call into dlcservice_util for `--list` option and
// will dump the output at the given path. If the path already exists, it will
// return an error.
func listDlcs(ctx context.Context, path string) error {
	// Path already exists.
	if _, err := os.Stat(path); err == nil {
		return errors.Wrapf(err, "file already exists at: %v", path)
	}
	cmd := testexec.CommandContext(ctx, "dlcservice_util", "--list", "--dump="+path)
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to list installed DLCs")
	}
	return nil
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

// GetInstalled calls the DBus methods to get installed DLCs.
func GetInstalled(ctx context.Context) ([]ListOutput, error) {
	testing.ContextLog(ctx, "Getting installed DLCs")
	path, err := ioutil.TempFile("", "get_installed")
	if err != nil {
		return nil, err
	}
	// listDlcs needs a non-existent file.
	os.Remove(path.Name())
	defer os.Remove(path.Name())
	if err := listDlcs(ctx, path.Name()); err != nil {
		return nil, err
	}
	m, err := dlcList(path.Name())
	if err != nil {
		return nil, err
	}
	installedIDs := make([]ListOutput, 0)
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

// Install calls the DBus method to install a DLC.
func Install(ctx context.Context, id, omahaURL string) error {
	testing.ContextLog(ctx, "Installing DLC: ", id, " using ", omahaURL)
	if err := testexec.CommandContext(ctx, "dlcservice_util", "--install", "--id="+id, "--omaha_url="+omahaURL).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to install")
	}
	return nil
}

// Purge calls the DBus method to Purge a DLC.
func Purge(ctx context.Context, id string) error {
	testing.ContextLog(ctx, "Purging DLC: ", id)
	if err := testexec.CommandContext(ctx, "dlcservice_util", "--purge", "--id="+id).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to purge")
	}
	return nil
}
