// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/testexec"
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
	androidFileContexts = "/etc/selinux/arc/contexts/files/android_file_contexts"
	dataDirFileContexts = "/tmp/data_file_contexts"
	replaceRawString    = `s|/data|/home/.shadow/[a-z0-9]*/mount/root/android-data/data|g`
)

func createFile(ctx context.Context) error {

	// Create a temporary SELinux Policy File
	cmd := fmt.Sprintf("grep '^/data' %s > %s", androidFileContexts, dataDirFileContexts)
	if _, err := testexec.CommandContext(ctx,
		"sh", "-c", cmd).Output(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to create temporary SELinux policy file")
	}

	// Correct paths in temporary SELinux policy file
	if _, err := testexec.CommandContext(ctx,
		"sed", "-i", replaceRawString, dataDirFileContexts).Output(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to correct paths in SELinux policy file")
	}
	return nil
}

func verifyDirSELinuxContext(ctx context.Context, directoryPath string) error {
	// Count the total files and sub-dirs in the dir
	filesCountCmd := fmt.Sprintf("find %s | wc -l", directoryPath)
	filesCount, err := testexec.CommandContext(ctx,
		"sh", "-c", filesCountCmd).Output(testexec.DumpLogOnError)
	if err != nil {
		return err
	}

	// Count the total number of SELinux context verified files and sub-dirs in the dir
	verifyCountCmd := fmt.Sprintf("find %s | xargs matchpathcon -f %s -V | grep verified | wc -l", directoryPath, dataDirFileContexts)
	verifiedCount, err := testexec.CommandContext(ctx,
		"sh", "-c", verifyCountCmd).Output(testexec.DumpLogOnError)
	if err != nil {
		return err
	}
	testing.ContextLogf(ctx, "Files Count = %v, Verified Count = %v", string(filesCount), string(verifiedCount))
	// All the files should be verified, else trigger an error
	if !bytes.Equal(filesCount, verifiedCount) {
		err := errors.Errorf("SELinux context verification failed for %v", directoryPath)
		return err
	}
	return nil
}

func SELinuxFilesDataDir(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(arc.PreData).Chrome

	// Create the temporarySELinux policy file
	s.Log("Creating the temporary SELinux context file for matchpathcon command")
	if err := createFile(ctx); err != nil {
		s.Fatal("Failed to successfully create SELinux policy file: ", err)
	}
	defer func() {
		if err := os.Remove(dataDirFileContexts); err != nil {
			s.Fatalf("Failed to remove file %s: %v", dataDirFileContexts, err)
		}
	}()

	//TODO(vraheja): Investigate why SELinux contexts mismatch for few dirs. http://b/162202740
	// List of /android-data/data/ directories to be checked
	dirList := []string{
		"adb",
		"anr",
		"app",
		"app-asec",
		"app-ephemeral",
		"app-lib",
		"app-private",
		"backup",
		"bootchart",
		"cache",
		"cache",
		"drm",
		"local",
		"lost+found",
		"media-drm",
		"misc_ce",
		"misc_de",
		"nfc",
		"ota",
		"ota_package",
		"property",
		"resource-cache",
		"ss",
		"system",
		"system_de",
		"tombstones",
		"user",
		"vendor",
		"vendor_ce",
		"vendor_de",
	}

	// Verify each dir for correct SELinux contexts
	ownerID, err := cryptohome.UserHash(ctx, cr.User())
	if err != nil {
		s.Fatal("Failed to get user hash: ", err)
	}

	dataDirPath := filepath.Join("/home/.shadow/", ownerID, "/mount/root/android-data/data")
	for _, dirName := range dirList {
		directoryPath := filepath.Join(dataDirPath, dirName)
		if err := verifyDirSELinuxContext(ctx, directoryPath); err != nil {
			s.Fatal("SELinux verification error: ", err)
		}
	}
}
