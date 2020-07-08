// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
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
)

func verifyDirSelinuxContext(ctx context.Context, s *testing.State, directoryPath string) error {
	// Count the total files and sub-dirs in the dir
	filesCountCmd := fmt.Sprintf("find %s | wc -l", directoryPath)
	filesCount, err := testexec.CommandContext(ctx,
		"sh", "-c", filesCountCmd).Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to count total files and sub-dirs in the dir: ", err)
	}

	// Count the total number of SELinux context verified files and sub-dirs in the dir
	verifyCountCmd := fmt.Sprintf("find %s | xargs matchpathcon -f %s -V | grep verified | wc -l", directoryPath, dataDirFileContexts)
	verifiedCount, err := testexec.CommandContext(ctx,
		"sh", "-c", verifyCountCmd).Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to count verified files and sub-dirs in the dir: ", err)
	}

	// All the files should be verified, else trigger an error
	if !bytes.Equal(filesCount, verifiedCount) {
		err := errors.Errorf("Selinux context verification failed for %v", directoryPath)
		return err
	}
	return nil
}

func SELinuxFilesDataDir(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.ARCEnabled())
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	// Create a temporary SeLinux Policy File
	s.Log("Creating the temporary SELinux context file for matchpathcon command")
	cmd := fmt.Sprintf("grep '/data' %s > %s", androidFileContexts, dataDirFileContexts)
	_, err = testexec.CommandContext(ctx,
		"sh", "-c", cmd).Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to create temporary Selinux Policy File: ", err)
	}

	// Correct paths in temporary SELinux policy file
	replaceRawString := `s/\/data/\/home\/.shadow\/[a-z0-9]*\/mount\/root\/android-data\/data/g`
	_, err = testexec.CommandContext(ctx,
		"sed", "-i", replaceRawString, dataDirFileContexts).Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to correct paths in the Selinux policy file: ", err)
	}

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
		"dalvik-cache",
		"cache",
		"drm",
		"local",
		"lost+found",
		"media-drm",
		//"misc",
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
	const dirTmpl = "/home/.shadow/%s/mount/root/android-data/data/"
	dataDirPath := fmt.Sprintf(dirTmpl, ownerID)
	for _, dirName := range dirList {
		directoryPath := dataDirPath + dirName
		if err := verifyDirSelinuxContext(ctx, s, directoryPath); err != nil {
			s.Fatal("Selinux verification error: ", err)
		}
	}

	// Remove the temporary file used to store SELinux contexts
	if err = os.Remove(dataDirFileContexts); err != nil {
		s.Fatalf("Failed to remove file %s: %v", dataDirFileContexts, err)
	}
}
