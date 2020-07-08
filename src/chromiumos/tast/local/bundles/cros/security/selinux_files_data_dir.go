// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SELinuxFilesDataDir,
		Desc:         "Checks SELinux labels specifically for the data dir in android-data",
		Contacts:     []string{"vraheja@chromium.org", "chromeos-security@google.com"},
		SoftwareDeps: []string{"selinux", "chrome"},
		Pre:          arc.Booted(),
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		Attr:    []string{"group:mainline", "informational"},
		Timeout: 5 * time.Minute,
	})
}

const (
	androidFileContexts  = "/etc/selinux/arc/contexts/files/android_file_contexts_1"
	dataDirFileContexts  = "/tmp/data_file_contexts"
	replaceRawString     = `s|/data|/home/.shadow/[a-z0-9]*/mount/root/android-data/data|g`
	matchPathConFileName = `matchpath_con_output`
)

func createSELinuxPolicyFile(ctx context.Context) error {
	// Create a temporary SELinux Policy File.
	cmdOut, err := testexec.CommandContext(ctx, "grep", `^/data`, androidFileContexts).Output(testexec.DumpLogOnError)
	if err != nil {
		return errors.Wrap(err, "failed to find android-data/data SELinux contexts")
	}
	err = ioutil.WriteFile(dataDirFileContexts, []byte(cmdOut), 0644)
	if err != nil {
		return errors.Wrap(err, "failed to create temporary SELinux file")
	}

	// Correct paths in temporary SELinux policy file.
	if _, err := testexec.CommandContext(ctx,
		"sed", "-i", replaceRawString, dataDirFileContexts).Output(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to correct paths in SELinux policy file")
	}
	return nil
}

func verifyDirSELinuxContext(ctx context.Context, directoryPath, outDir string) error {
	// Count the total number of SELinux context verified files and sub-dirs in the dir.
	matchPathConCmd := fmt.Sprintf("find %s -exec matchpathcon -f %s -V {} \\;", shutil.Escape(directoryPath), shutil.Escape(dataDirFileContexts))
	matchPathConOutput, err := testexec.CommandContext(ctx,
		"sh", "-c", matchPathConCmd).Output(testexec.DumpLogOnError)
	if err != nil {
		return err
	}
	matchPathConStr := string(matchPathConOutput)
	linesCount := strings.Count(matchPathConStr, "\n")
	verifiedCount := strings.Count(matchPathConStr, "verified.\n")
	// Write the output of matchpathcon command.
	matchPathConFileLocation := filepath.Join(outDir, matchPathConFileName)
	if err := ioutil.WriteFile(matchPathConFileLocation,
		[]byte(matchPathConOutput), 0644); err != nil {
		return errors.Wrap(err, "failed to write matchpathcon output")
	}

	//testing.ContextLog(ctx, "MatchPathCon File = ", matchPathConFileLocation)
	if linesCount != verifiedCount {
		err := errors.Errorf("SELinux context verification failed for %v", directoryPath)
		return err
	}

	// Remove the mathcpathcon output file if SELinux verification succeeds.
	if err = os.Remove(matchPathConFileLocation); err != nil {
		return errors.Wrapf(err, "failed to remove file %s", matchPathConFileLocation)
	}

	return nil
}

func SELinuxFilesDataDir(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(arc.PreData).Chrome

	// Create the temporarySELinux policy file.
	s.Log("Creating the temporary SELinux context file for matchpathcon command")
	if err := createSELinuxPolicyFile(ctx); err != nil {
		s.Fatal("Failed to successfully create SELinux policy file: ", err)
	}

	defer func() {
		if err := os.Remove(dataDirFileContexts); err != nil {
			s.Fatalf("Failed to remove file %s: %v", dataDirFileContexts, err)
		}
	}()

	// Verify SELinux context for all files and directories except those in the skipDirMap.
	ownerID, err := cryptohome.UserHash(ctx, cr.User())
	if err != nil {
		s.Fatal("Failed to get user hash: ", err)
	}
	dataDirPath := filepath.Join("/home/.shadow", ownerID, "mount/root/android-data/data")
	dirList, err := ioutil.ReadDir(dataDirPath)
	if err != nil {
		s.Fatalf("Failed to read from directory %v: %v", dataDirPath, err)
	}
	//TODO(vraheja): Investigate why SELinux contexts mismatch for few dirs. http://b/162202740
	skipDirMap := map[string]struct{}{
		"data":    {},
		"media":   {},
		"user_de": {},
	}

	for _, dir := range dirList {
		// If the directory exists in the skip directory map, do not verify its SELinux context.
		if _, exists := skipDirMap[dir.Name()]; exists {
			continue
		}
		directoryPath := filepath.Join(dataDirPath, dir.Name())
		if err := verifyDirSELinuxContext(ctx, directoryPath, s.OutDir()); err != nil {
			s.Fatalf("SELinux verification error: %v (see file - %s in test output directory for details)", err, matchPathConFileName)
		}
	}
}
