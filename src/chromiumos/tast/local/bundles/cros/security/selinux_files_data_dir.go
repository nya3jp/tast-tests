// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/cryptohome"
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
		Attr:    []string{"group:mainline"},
		Timeout: 5 * time.Minute,
	})
}

const (
	androidFileContextsP  = "/etc/selinux/arc/contexts/files/android_file_contexts"
	androidFileContextsVM = "/etc/selinux/arc/contexts/files/android_file_contexts_vm"
	dataDirFileContexts   = "/tmp/data_file_contexts"
	androidDataPath       = "/home/.shadow/[a-z0-9]*/mount/root/android-data"
	dataPrefix            = "/data"
	matchPathConFileName  = `matchpath_con_output`
	selinuxRegexCapture   = `(\w:\w*:\w*:\w*)\S*`
)

func createSELinuxPolicyFile(ctx context.Context) error {
	androidFileContexts := androidFileContextsP
	if vmEnabled, err := arc.VMEnabled(); err != nil {
		return errors.Wrap(err, "failed to check whether ARCVM is enabled")
	} else if vmEnabled {
		androidFileContexts = androidFileContextsVM
	}
	// Read SELinux android file contexts and find the lines starting with /data.
	f, err := os.Open(androidFileContexts)
	if err != nil {
		return errors.Wrap(err, "failed to open SELinux file - android_file_contexts")
	}
	defer f.Close()

	fout := &bytes.Buffer{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, dataPrefix) {
			continue
		}
		dataLine := androidDataPath + line
		fmt.Fprintln(fout, dataLine)
	}
	if sc.Err() != nil {
		return errors.Wrap(sc.Err(), "failed to read from SELinux file - android_file_contexts")
	}

	// Create a temporary SELinux Policy File.
	if err = ioutil.WriteFile(dataDirFileContexts, fout.Bytes(), 0644); err != nil {
		return errors.Wrap(err, "failed to create temporary SELinux file")
	}
	return nil
}

func verifyDirSELinuxContext(ctx context.Context, directoryPath, outDir string) error {
	matchPathConOutput, err := testexec.CommandContext(ctx, "find", directoryPath, "-exec", "matchpathcon", "-f", dataDirFileContexts, "-V", "{}", ";").Output(testexec.DumpLogOnError)
	if err != nil {
		return err
	}
	matchPathConStr := string(matchPathConOutput)
	linesCount := strings.Count(matchPathConStr, "\n")
	verifiedCount := strings.Count(matchPathConStr, "verified.\n")
	// Counts any files or dirs which are not found during the race between
	// find and matchpathcon command.
	filesAndDirMissingCount := strings.Count(matchPathConStr, "error: No such file or directory\n")

	// Ruling out False-negative SELinux mismatches due to multicategory part.
	scanner := bufio.NewScanner(strings.NewReader(matchPathConStr))
	for scanner.Scan() {
		line := scanner.Text()
		// If the matchpathcon output doesn't contain "verified", it means
		// verification failed. We further need to investigate if it
		// failed due to SELinux multi-category part not matching.
		if !strings.Contains(line, "verified") {

			// Build regex for SELinux context.
			// selinuxRegexp captures SELinux contexts like "u:object_r:shared_relro_file:s0".
			selinuxRegexp := regexp.MustCompile(selinuxRegexCapture)
			if matches := selinuxRegexp.FindAllString(line, -1); len(matches) == 2 {
				// For a mismatch of SELinux context, get the value of
				// actual and expected context from the output.
				actualContext := matches[0]
				expectedContext := matches[1]
				// We can ignore the multi-category part if the
				// expected context is a prefix of actual context.
				// Example:
				// /home/.shadow/*/libwebviewchromium32.relro has context
				// u:object_r:shared_relro_file:s0:c13,c260,c512,c768,
				// should be u:object_r:shared_relro_file:s0
				if strings.HasPrefix(actualContext, expectedContext) {
					verifiedCount = verifiedCount + 1
				}
			}
		}
	}
	if scanner.Err() != nil {
		return errors.Wrap(scanner.Err(), "failed to scan lines of matchpathcon output")
	}

	// Write the output of matchpathcon command.
	matchPathConFileLocation := filepath.Join(outDir, matchPathConFileName)
	if err := ioutil.WriteFile(matchPathConFileLocation,
		[]byte(matchPathConOutput), 0644); err != nil {
		return errors.Wrap(err, "failed to write matchpathcon output")
	}
	// Every line pointing to a file in matchpathcon output should either be verified,
	// or the file isn't found due to race between find and matchpathcon command.
	if linesCount != verifiedCount+filesAndDirMissingCount {
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
	ownerID, err := cryptohome.UserHash(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to get user hash: ", err)
	}
	dataDirPath := filepath.Join("/home/.shadow", ownerID, "mount/root/android-data/data")
	dirList, err := ioutil.ReadDir(dataDirPath)
	if err != nil {
		s.Fatalf("Failed to read from directory %v: %v", dataDirPath, err)
	}
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
