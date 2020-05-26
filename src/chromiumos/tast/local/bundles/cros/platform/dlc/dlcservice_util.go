// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package dlc provides interfacing with dlcservice daemon.
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

type dlcListOutput struct {
	RootMount string `json:"root_mount"`
}

func verifyDlcContent(s *testing.State, path, dlc string) {
	readFile := func(path string) []byte {
		b, err := ioutil.ReadFile(path)
		if err != nil {
			s.Fatal("Failed to read file: ", err)
		}
		return b
	}
	removeExt := func(path string) string {
		return strings.TrimSuffix(path, filepath.Ext(path))
	}
	checkSHA2Sum := func(hash_path string) {
		path := removeExt(hash_path)
		actualSumBytes := sha256.Sum256(readFile(path))
		actualSum := hex.EncodeToString(actualSumBytes[:])
		expectedSum := strings.Fields(string(readFile(hash_path)))[0]
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
		expectedPerm := strings.TrimSpace(string(readFile(perms_path)))
		if actualPerm != expectedPerm {
			s.Fatalf("Permissions do not match for %s. Actual=%s Expected=%s",
				path, actualPerm, expectedPerm)
		}
	}
	dlcList := func(path string) (output map[string][]dlcListOutput) {
		if err := json.Unmarshal(readFile(path), &output); err != nil {
			s.Fatal("Failed to read json: ", err)
		}
		return output
	}

	getRootMounts := func(path, dlc string) (rootMounts []string) {
		if l, ok := dlcList(path)[dlc]; ok {
			for _, val := range l {
				rootMounts = append(rootMounts, val.RootMount)
			}
		}
		return rootMounts
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

func runCmd(ctx context.Context, s *testing.State, msg, name string, args ...string) {
	cmd := testexec.CommandContext(ctx, name, args...)
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to ", msg, err)
	}
}

// DumpAndVerifyInstalledDLCs calls dlcservice's GetInstalled D-Bus method
// via dlcservice_util command.
func DumpAndVerifyInstalledDLCs(ctx context.Context, s *testing.State, tag string, dlcs ...string) {
	s.Log("Asking dlcservice for installed DLC modules")
	f := tag + ".log"
	path := filepath.Join(s.OutDir(), f)
	cmd := testexec.CommandContext(ctx, "dlcservice_util", "--list", "--dump="+path)
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to get installed DLC modules: ", err)
	}
	for _, dlc := range dlcs {
		verifyDlcContent(s, path, dlc)
	}
}

// Install calls the DBus method to install a DLC.
func Install(ctx context.Context, s *testing.State, dlc, omahaURL string) {
	s.Log("Installing DLC: ", dlc, " using ", omahaURL)
	runCmd(ctx, s, "install", "dlcservice_util", "--install", "--id="+dlc, "--omaha_url="+omahaURL)
}

// Uninstall calls the DBus method to uninstall a DLC.
func Uninstall(ctx context.Context, s *testing.State, dlc string) {
	s.Log("Uninstalling DLC: ", dlc)
	runCmd(ctx, s, "uninstall", "dlcservice_util", "--uninstall", "--id="+dlc)
}
