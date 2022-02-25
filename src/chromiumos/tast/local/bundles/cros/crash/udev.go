// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"compress/gzip"
	"context"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/crash"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

const (
	udevBaseName = `__tast_udev_crash_test\..*`
	udevLogName  = udevBaseName + `\.log.*` // match .log and .log.gz
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     Udev,
		Desc:     "Verify udev triggered crash works as expected",
		Contacts: []string{"yamaguchi@chromium.org", "iby@chromium.org", "cros-telemetry@google.com"},
		Attr:     []string{"group:mainline"},
	})
}

// readLog reads file given by filename, possibly decoding gzip.
func readLog(filename string) ([]byte, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var r io.Reader = f
	if strings.HasSuffix(filename, ".gz") {
		r, err = gzip.NewReader(f)
		if err != nil {
			return nil, err
		}
	}
	return ioutil.ReadAll(r)
}

func Udev(ctx context.Context, s *testing.State) {
	if err := crash.SetUpCrashTest(ctx, crash.WithMockConsent()); err != nil {
		s.Fatal("SetUpCrashTest failed: ", err)
	}
	defer crash.TearDownCrashTest(ctx)

	// Use udevadm to trigger a test-only udev event representing driver failure.
	// See 99-crash-reporter.rules for the matcing udev rule.

	s.Log("Triggering a fake crash event via udev")

	for _, args := range [][]string{
		{"udevadm", "control", "--property=TAST_UDEV_TEST=crash"},
		{"udevadm", "trigger", "-p", "DEVNAME=/dev/mapper/control"},
		{"udevadm", "control", "--property=TAST_UDEV_TEST="},
	} {
		if err := testexec.CommandContext(ctx, args[0], args[1:]...).Run(); err != nil {
			s.Fatalf("%s failed: %v", shutil.EscapeSlice(args), err)
		}
	}

	s.Log("Waiting for the corresponding crash report")

	crashDirs, err := crash.GetDaemonStoreCrashDirs(ctx)
	if err != nil {
		s.Fatal("Couldn't get daemon store dirs: ", err)
	}
	// We might not be logged in, so also allow system crash dir.
	crashDirs = append(crashDirs, crash.SystemCrashDir)

	expectedRegexes := []string{udevBaseName + `\.meta`, udevLogName}

	// Check proper crash reports are created.
	files, err := crash.WaitForCrashFiles(ctx, crashDirs, expectedRegexes)
	if err != nil {
		s.Fatal("Couldn't find expected files: ", err)
	}
	defer func() {
		if err := crash.RemoveAllFiles(ctx, files); err != nil {
			s.Log("Couldn't clean up files: ", err)
		}
	}()

	if len(files[udevLogName]) >= 1 {
		found := false
		for _, file := range files[udevLogName] {
			b, err := readLog(file)
			if err != nil {
				// try later log files -- but this shouldn't happen, so still error.
				s.Error("Failed reading log file: ", err)
				continue
			}
			if string(b) != "ok\n" {
				continue
			}
			found = true
			break
		}
		if !found {
			s.Error("No matching log file found")
		}
	} else {
		s.Error("No log files in crash report")
	}
}
