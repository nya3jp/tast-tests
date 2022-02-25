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
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
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

func checkFakeCrashes(files map[string][]string) (bool, error) {
	if len(files[udevLogName]) >= 1 {
		for _, file := range files[udevLogName] {
			b, err := readLog(file)
			if err != nil {
				// Content error of .gz file (e.g. Unexpected EOF) can happen when
				// the file has not been written to the end. Skip it so that the
				// file can be visited again by the polling.
				continue
			}
			if string(b) != "ok\n" {
				continue
			}
			return true, nil
		}
	} else {
		return false, errors.New("Missing log file")
	}
	return false, nil
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

	files, err := crash.WaitForCrashFiles(ctx, crashDirs, expectedRegexes)
	if err != nil {
		s.Fatal("Couldn't find expected files: ", err)
	}
	defer func() {
		if err := crash.RemoveAllFiles(ctx, files); err != nil {
			s.Log("Couldn't clean up files: ", err)
		}
	}()

	// Check proper crash reports are created.
	err = testing.Poll(ctx, func(c context.Context) error {
		found, err := checkFakeCrashes(files)
		if err != nil {
			s.Fatal("Failed while polling crash log: ", err)
		}
		if !found {
			return errors.New("no fake crash found")
		}
		return nil
	}, &testing.PollOptions{Timeout: 60 * time.Second})
	if err != nil {
		s.Error("Failed to wait for crash reports: ", err)
	}

}
