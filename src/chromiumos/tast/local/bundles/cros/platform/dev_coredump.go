// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

const (
	iwlwifiDir = "/sys/kernel/debug/iwlwifi"
	crashDir   = "/var/spool/crash"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     DevCoredump,
		Desc:     "Verify device coredumps are handled as expected",
		Contacts: []string{"mwiitala@google.com", "cros-monitoring-forensics@google.com"},
		Attr:     []string{"group:mainline", "informational"},
		// TODO: Check for a dependency or attribute group I could use to check if device has intel WiFi
		SoftwareDeps: []string{"wifi"},
	})
}

func checkForDevCoredump(existingFiles map[string]struct{}) (bool, error) {
	files, err := ioutil.ReadDir(crashDir)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	for _, file := range files {
		filename := file.Name()
		if _, found := existingFiles[filename]; found {
			continue
		}
		if strings.HasPrefix(filename, "devcoredump_iwlwifi.") && strings.HasSuffix(filename, ".devcore") {
			return true, nil
		}
	}
	return false, nil
}

func DevCoredump(ctx context.Context, s *testing.State) {
	// Memorize existing crash files to distinguish new files from them.
	files, err := ioutil.ReadDir(crashDir)
	existingFiles := make(map[string]struct{})
	if err != nil && !os.IsNotExist(err) {
		s.Fatal("Failed to read system crash dir: ", err)
	}
	for _, file := range files {
		existingFiles[file.Name()] = struct{}{}
	}

	// Verify that DUT has Intel WiFi.
	if _, err = os.Stat(iwlwifiDir); os.IsNotExist(err) {
		// TODO: Remove this check if Intel Wifi dependency exists to skip running test entirely.
		s.Log("iwlwifi directory does not exist on DUT, skipping test")
		return
	}

	s.Log("Triggering a devcoredump by restarting wifi firmware")

	// Use the find command to get the full path to the fw_restart file.
	path, err := testexec.CommandContext(ctx, "find", iwlwifiDir, "-name", "fw_restart").Output()
	if err != nil {
		s.Fatal("Failed to find fw_restart file: ", err)
	}

	// Trigger a wifi fw restart by echoing 1 into the fw_restart file.
	err = testexec.CommandContext(ctx, "sh", "-c", string("echo 1 > "+string(path))).Run()
	if err != nil {
		s.Fatal("Failed to trigger device coredump: ", err)
	}

	s.Log("Waiting for .devcore file to be added to crash directory")

	// Check expected device coredump is copied to crash directory.
	err = testing.Poll(ctx, func(c context.Context) error {
		found, err := checkForDevCoredump(existingFiles)
		if err != nil {
			s.Fatal("Failed while polling crash directory: ", err)
		}
		if !found {
			return errors.New("no .devcore file found")
		}
		return nil
	}, &testing.PollOptions{Timeout: 60 * time.Second})
	if err != nil {
		s.Error("Failed to wait for device coredump: ", err)
	}
}
