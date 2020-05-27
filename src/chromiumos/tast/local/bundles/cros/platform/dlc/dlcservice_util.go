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

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// ListOutput holds the output from running `dlcservice_util --list`.
type ListOutput struct {
	ID        string `json:"id"`
	Package   string `json:"package"`
	RootMount string `json:"root_mount"`
}

// readFile returns the bytes within the file at the given path.
func readFile(s *testing.State, path string) []byte {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		s.Fatal("Failed to read file: ", err)
	}
	return b
}

// dlcList converts the file at path to a ListOutput.
func dlcList(s *testing.State, path string) (listOutput map[string][]ListOutput) {
	if err := json.Unmarshal(readFile(s, path), &listOutput); err != nil {
		s.Fatal("Failed to read json: ", err)
	}
	return
}

// verifyDlcContent verifies that the contents of the DLC have valid file hashes and file permissions.
func verifyDlcContent(s *testing.State, path, dlc string) {
	removeExt := func(path string) string {
		return strings.TrimSuffix(path, filepath.Ext(path))
	}

	checkSHA2Sum := func(hash_path string) {
		path := removeExt(hash_path)
		actualSumBytes := sha256.Sum256(readFile(s, path))
		actualSum := hex.EncodeToString(actualSumBytes[:])
		expectedSum := strings.Fields(string(readFile(s, hash_path)))[0]
		if actualSum != expectedSum {
			s.Fatalf("SHA2 checksum do not match for %s. Actual=%s Expected=%s",
				path, actualSum, expectedSum)
		}
	}

	checkPerms := func(perms_path string) {
		path := removeExt(perms_path)
		info, err := os.Stat(path)
		if err != nil {
			s.Fatal("Failed to stat: ", err)
		}
		actualPerm := fmt.Sprintf("%#o", info.Mode().Perm())
		expectedPerm := strings.TrimSpace(string(readFile(s, perms_path)))
		if actualPerm != expectedPerm {
			s.Fatalf("Permissions do not match for %s. Actual=%s Expected=%s",
				path, actualPerm, expectedPerm)
		}
	}

	getRootMounts := func(path, dlc string) (rootMounts []string) {
		if l, ok := dlcList(s, path)[dlc]; ok {
			for _, val := range l {
				rootMounts = append(rootMounts, val.RootMount)
			}
		}
		return
	}

	rootMounts := getRootMounts(path, dlc)
	if len(rootMounts) == 0 {
		s.Fatal("Failed to get root mount for ", dlc)
	}
	for _, rootMount := range rootMounts {
		filepath.Walk(rootMount, func(path string, info os.FileInfo, err error) error {
			switch filepath.Ext(path) {
			case ".sum":
				checkSHA2Sum(path)
				break
			case ".perms":
				checkPerms(path)
				break
			}
			return nil
		})
	}
}

// listDlcs is a helper to call into dlcservice_util for `--list` option.
func listDlcs(ctx context.Context, s *testing.State, path string) {
	// Path already exists.
	if _, err := os.Stat(path); err == nil {
		s.Fatal("File already exists at: ", path)
	}
	cmd := testexec.CommandContext(ctx, "dlcservice_util", "--list", "--dump="+path)
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to list DLCs: ", err)
	}
}

// DumpAndVerifyInstalledDLCs calls dlcservice's GetInstalled D-Bus method
// via dlcservice_util command.
func DumpAndVerifyInstalledDLCs(ctx context.Context, s *testing.State, tag string, dlcs ...string) {
	s.Log("Asking dlcservice for installed DLC modules")
	f := tag + ".log"
	path := filepath.Join(s.OutDir(), f)
	listDlcs(ctx, s, path)
	for _, dlc := range dlcs {
		verifyDlcContent(s, path, dlc)
	}
}

// GetInstalled calls the DBus methods to get installed DLCs.
func GetInstalled(ctx context.Context, s *testing.State, path string) []ListOutput {
	s.Log("Getting installed DLCs")
	listDlcs(ctx, s, path)
	m := dlcList(s, path)
	installedIDs := make([]ListOutput, 0)
	for id, l := range m {
		for _, val := range l {
			if id != val.ID {
				s.Errorf("List has mismatching IDs: %s %s", id, val.ID)
				continue
			}
			if val.Package == "" {
				s.Errorf("Empty package for ID: %s", id)
				continue
			}
			installedIDs = append(installedIDs, val)
		}
	}
	return installedIDs
}

// Install calls the DBus method to install a DLC.
func Install(ctx context.Context, s *testing.State, dlc, omahaURL string) {
	s.Log("Installing DLC: ", dlc, " using ", omahaURL)
	if err := testexec.CommandContext(ctx, "dlcservice_util", "--install", "--id="+dlc, "--omaha_url="+omahaURL).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to install: ", err)
	}
}

// Uninstall calls the DBus method to uninstall a DLC.
func Uninstall(ctx context.Context, s *testing.State, dlc string) {
	s.Log("Uninstalling DLC: ", dlc)
	if err := testexec.CommandContext(ctx, "dlcservice_util", "--uninstall", "--id="+dlc).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to install: ", err)
	}
}
