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
	"reflect"
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

	testing.ContextLog(ctx, "Lines Count = ", linesCount)
	testing.ContextLog(ctx, "Verified Count = ", verifiedCount)
	testing.ContextLog(ctx, "Files and Missing Count = ", filesAndDirMissingCount)

	// Here ------ Vaibhav Raheja
	// Vaibhav - starting from here
	scanner := bufio.NewScanner(strings.NewReader(string(matchPathConOutput)))
	//testing.ContextLog(ctx, "Vaibhav Raheja - printing the lines")
	for scanner.Scan() {

		line := scanner.Text()
		both := strings.Contains(line, "has context") && strings.Contains(line, "should be")
		if both == true {
			testing.ContextLog(ctx, "Vaibhav Raheja - ", scanner.Text(), "\n")
			hasContextIndex := strings.Index(line, "has context")
			shouldBeIndex := strings.Index(line, "should be")
			//newLinesIndex := strings.Index(line, "\n")
			testing.ContextLog(ctx, "Has context Index = ", hasContextIndex, "\n")
			testing.ContextLog(ctx, "Should Be Index = ", shouldBeIndex, "\n")
			testing.ContextLog(ctx, "Total Length = ", len(line), "\n")

			actual := string(line[hasContextIndex+12 : shouldBeIndex-2])
			testing.ContextLog(ctx, "Actual Context = ", actual, "\n")

			expected := string(line[shouldBeIndex+10:])
			testing.ContextLog(ctx, "Expected Context = ", expected, "\n")

			actualSlice := strings.Split(actual, ":")
			expectedSlice := strings.Split(expected, ":")

			testing.ContextLog(ctx, "Actual Slice Length = ", len(actualSlice), "\n")
			testing.ContextLog(ctx, "Expected Slice Length = ", len(expectedSlice), "\n")

			if len(actualSlice) > 0 {
				actualSlice = actualSlice[:len(actualSlice)-1]
			}

			firstFourEqual := reflect.DeepEqual(actualSlice, expectedSlice)
			testing.ContextLog(ctx, "First Four Equal = ", firstFourEqual, "\n")

			if firstFourEqual == true {
				verifiedCount = verifiedCount + 1
			}

		}

	}
	err = scanner.Err()

	testing.ContextLog(ctx, "Lines Count After = ", linesCount)
	testing.ContextLog(ctx, "Verified Count After= ", verifiedCount)
	testing.ContextLog(ctx, "Files and Missing Count After= ", filesAndDirMissingCount)
	// Vaibhav - ends here

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
	//TODO(vraheja): Investigate why SELinux contexts mismatch for few dirs. http://b/162202740
	skipDirMap := map[string]struct{}{
		"data":    {},
		"media":   {},
		"user_de": {},
		//"misc":    {},
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
