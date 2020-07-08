// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CheckFiles,
		Desc:         "Checks SELinux labels specifically for the data dir in android-data",
		Contacts:     []string{"vraheja@chromium.org", "chromeos-security@google.com"},
		SoftwareDeps: []string{"android_p", "selinux", "chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      5 * time.Minute,
	})
}

func verifyDirContext(ctx context.Context, s *testing.State, dirName string) {

	dataDirFileContexts := "/tmp/data_file_contexts"
	basePath := "/home/.shadow/[a-z0-9]*/mount/root/android-data/data/"
	totalPath := basePath + dirName

	// Check that totalPath must be a reachable location

	// Count the total files and sub-dirs in the dir
	filesCountCmd := fmt.Sprintf("find %s | wc -l", totalPath)
	filesCount, err := testexec.CommandContext(ctx,
		"sh", "-c", filesCountCmd).Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to count total files and sub-dirs in the Dir: ", err)
	}

	// Count the total number of SELinux context verified files and sub-dirs in the dir
	verifyCountCmd := fmt.Sprintf("find %s | xargs matchpathcon -f %s -V | grep verified | wc -l", totalPath, dataDirFileContexts)
	verifiedCount, err := testexec.CommandContext(ctx,
		"sh", "-c", verifyCountCmd).Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to count verified files and sub-dirs in the Dir: ", err)
	}

	if bytes.Compare(filesCount, verifiedCount) != 0 {
		s.Log("Mismatch in verified files for dir - ", dirName)
	} else {
		s.Log("All verified for dir - ", dirName)
	}
}

func CheckFiles(ctx context.Context, s *testing.State) {

	s.Log("Test Code starts")

	androidFileContexts := "/etc/selinux/arc/contexts/files/android_file_contexts"
	dataDirFileContexts := "/tmp/data_file_contexts"

	// 1. Create the SeLinux Policy File
	cmd := fmt.Sprintf("grep '/data' %s > %s", androidFileContexts, dataDirFileContexts)
	_, err := testexec.CommandContext(ctx,
		"sh", "-c", cmd).Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed create the Policy File: ", err)
	}

	// 2. Modify the SeLinux Policy File
	replaceRawString := `s/\/data/\/home\/.shadow\/[a-z0-9]*\/mount\/root\/android-data\/data/g`
	_, err = testexec.CommandContext(ctx,
		"sed", "-i", replaceRawString, dataDirFileContexts).Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to modify the policy file: ", err)
	}

	// 3. Create a list of directories to be checked
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
		"data",
		"drm",
		"local",
		"lost+found",
		"media",
		"media-drm",
		"misc",
		"misc_ce",
		"misc_de",
		"nfc",
		"ota",
		"ota_package",
		"property",
		"resource-cache",
		"ss",
		"system",
		"system_ce",
		"system_de",
		"tombstones",
		"user",
		"user_de",
		"vendor",
		"vendor_ce",
		"vendor_de",
	}

	// 4. Verify each dir using the function written above

	for _, dir := range dirList {
		verifyDirContext(ctx, s, dir)
	}

	s.Log("Test Code completes")
}
