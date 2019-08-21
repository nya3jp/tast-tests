// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"compress/gzip"
	"context"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/metrics"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

const (
	systemCrashDir = "/var/spool/crash"
	testCert       = "testcert.p12"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: UdevCrash,
		Desc: "Verify udev triggered crash works as expected",
		// TODO(yamaguchi): Add proper owner addresses.
		Contacts: []string{"yamaguchi@chromium.org"},
		Attr:     []string{"informational"},
		Data:     []string{testCert},
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

func checkFakeCrashes(pastCrashes map[string]struct{}) (bool, error) {
	files, err := ioutil.ReadDir(systemCrashDir)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	for _, file := range files {
		filename := file.Name()
		if _, found := pastCrashes[filename]; found {
			continue
		}
		if !strings.HasPrefix(filename, "__tast_udev_crash_test.") {
			continue
		}
		if !strings.HasSuffix(filename, ".log") && !strings.HasSuffix(filename, ".log.gz") {
			continue
		}
		b, err := readLog(filepath.Join(systemCrashDir, filename))
		if err != nil {
			return false, err
		}
		if string(b) != "ok\n" {
			continue
		}
		return true, nil
	}
	return false, nil
}

func UdevCrash(ctx context.Context, s *testing.State) {
	if err := metrics.SetConsent(ctx, s.DataPath(testCert)); err != nil {
		s.Fatal("Failed to set consent: ", err)
	}

	// Memorize existing cresh report to distinguish new reports from them.
	files, err := ioutil.ReadDir(systemCrashDir)
	pastCrashes := make(map[string]struct{})
	if err != nil && !os.IsNotExist(err) {
		s.Fatal("Failed to read system crash dir: ", err)
	}
	for _, file := range files {
		pastCrashes[file.Name()] = struct{}{}
	}

	// Use udevadm to trigger a fake udev event representing driver failure.
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

	// Check proper crash reports are created.
	err = testing.Poll(ctx, func(c context.Context) error {
		found, err := checkFakeCrashes(pastCrashes)
		if err != nil {
			s.Fatal("Failed while polling crash log: ", err)
		}
		if found {
			return nil
		}
		return errors.New("no fake crash found")
	}, &testing.PollOptions{Timeout: 60 * time.Second})
	if err != nil {
		s.Error("Failed to wait for crash reports: ", err)
	}
}
