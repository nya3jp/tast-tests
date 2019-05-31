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
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

const (
	mockMetricsOnPolicyFile = "udev_crash_mock_metrics_on.policy"
	mockMetricsOwnerKeyFile = "udev_crash_mock_metrics_owner.key"
	systemCrashDir          = "/var/spool/crash"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: UdevCrash,
		Desc: "Verify udev triggered crash works as expected",
		// TODO(yamaguchi): Add proper owner addresses.
		Contacts: []string{"yamaguchi@chromium.org"},
		Attr:     []string{"informational"},
		Data: []string{
			mockMetricsOnPolicyFile,
			mockMetricsOwnerKeyFile,
		},
	})
}

// setConsent sets whether or not we have consent to send crash reports.
// This creates or deletes the file to control whether
// crash_sender will consider that it has consent to send crash reports.
// It also copies a policy blob with the proper policy setting.
func setConsent(ctx context.Context, mockMetricsOnPolicyFilePath string,
	mockMetricsOwnerKeyFilePath string) error {
	const (
		whitelistDir     = "/var/lib/whitelist"
		consentFile      = "/home/chronos/Consent To Send Stats"
		onwerKeyFile     = whitelistDir + "/owner.key"
		signedPolicyFile = whitelistDir + "/policy"
	)
	if e, err := os.Stat(whitelistDir); err == nil && e.IsDir() {
		// Create policy file that enables metrics/consent.
		if err := fsutil.CopyFile(mockMetricsOnPolicyFilePath, signedPolicyFile); err != nil {
			return err
		}
		if err := fsutil.CopyFile(mockMetricsOwnerKeyFilePath, onwerKeyFile); err != nil {
			return err
		}
	}
	// Create deprecated consent file.  This is created *after* the
	// policy file in order to avoid a race condition where chrome
	// might remove the consent file if the policy's not set yet.
	// We create it as a temp file first in order to make the creation
	// of the consent file, owned by chronos, atomic.
	// See crosbug.com/18413.
	tempFile := consentFile + ".tmp"
	if err := ioutil.WriteFile(tempFile, []byte("test-consent"), 0644); err != nil {
		return err
	}

	if err := os.Chown(tempFile, int(sysutil.ChronosUID), int(sysutil.ChronosGID)); err != nil {
		return err
	}
	if err := os.Rename(tempFile, consentFile); err != nil {
		return err
	}
	testing.ContextLog(ctx, "Created ", consentFile)
	return nil
}

// checkLogContent reads file given by filename. complete is true if it's a valid log
// expected for the test. resultErr is set to non-nil if any error or verification error
// detected. Otherwise the log has not been written to the end.
func checkLogContent(filename string) (complete bool, resultErr error) {
	var r io.Reader
	if strings.HasSuffix(filename, ".log.gz") {
		f, err := os.Open(filename)
		if err != nil {
			resultErr = err
			return
		}
		defer f.Close()
		r, err = gzip.NewReader(f)
		if err != nil {
			resultErr = err
			return
		}
	} else if strings.HasSuffix(filename, ".log") {
		f, err := os.Open(filename)
		if err != nil {
			resultErr = err
			return
		}
		defer f.Close()
		r = f
	} else {
		resultErr = errors.Errorf("crash report %s has wrong extension", filename)
		return
	}

	lines, err := ioutil.ReadAll(r)
	if err != nil {
		resultErr = err
		return
	}
	// Check that we have seen the end of the file. Otherwise we could
	// end up racing bwtween writing to the log file and reading/checking
	// the log file.
	if !strings.Contains(string(lines), "END-OF-LOG") {
		return
	}

	for _, line := range strings.Split(string(lines), "\n") {
		if len(line) > 0 && !strings.Contains(line, "atmel_mxt_ts") && !strings.Contains(line, "END-OF-LOG") {
			resultErr = errors.Errorf("Crash report contains invalid content %q", line)
			return
		}
	}
	complete = true
	return
}

func checkAtmelCrashes(pastCrashes map[string]struct{}) (bool, error) {
	// Check proper Atmel trackpad crash reports are created.
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
		if !strings.HasPrefix(filename, "change__i2c_atmel_mxt_ts") ||
			strings.HasSuffix(filename, ".meta") {
			continue
		}
		path := filepath.Join(systemCrashDir, filename)
		result, err := checkLogContent(path)
		if err != nil {
			return false, err
		}
		if !result {
			continue
		}
		return true, nil
	}
	return false, nil
}

func hasAtmelDeviceDir() (hasDevice bool, resultErr error) {
	const driverDir = "/sys/bus/i2c/drivers/atmel_mxt_ts"

	if r, err := os.Stat(driverDir); err == nil && r.IsDir() {
		files, err := ioutil.ReadDir(driverDir)
		if err != nil {
			resultErr = errors.Wrap(err, "Failed to read Atmel driver dir")
		} else {
			for _, file := range files {
				if file.Mode()&os.ModeSymlink != 0 {
					fullpath, _ := filepath.EvalSymlinks(filepath.Join(driverDir, file.Name()))
					file, _ = os.Stat(fullpath)
				}
				if file.Mode().IsDir() {
					hasDevice = true
				}
			}
		}
	}
	return
}

func UdevCrash(ctx context.Context, s *testing.State) {
	hasDevice, err := hasAtmelDeviceDir()
	if err != nil {
		s.Fatal("Error occured while searching Atmel devices: ", err)
	}
	if !hasDevice {
		// TODO(yamaguchi): Change this to an error when hardware depenency is
		// supported by the test framework.
		s.Log("No Atmel device found; this test should not be run on this device")
	}

	if err := setConsent(ctx, s.DataPath(mockMetricsOnPolicyFile),
		s.DataPath(mockMetricsOwnerKeyFile)); err != nil {
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

	// Use udevadm to trigger a fake udev event representing atmel driver
	// failure. The uevent match rule in 99-crash-reporter.rules is
	// ACTION=="change", SUBSYSTEM=="i2c", DRIVER=="atmel_mxt_ts",
	// ENV{ERROR}=="1" RUN+="/sbin/crash_reporter
	// --udev=SUBSYSTEM=i2c-atmel_mxt_ts:ACTION=change"

	for _, args := range [][]string{
		{"udevadm", "control", "--property=ERROR=1"},
		{"udevadm", "trigger",
			"--action=change",
			"--subsystem-match=i2c",
			"--attr-match=driver=atmel_mxt_ts"},
		{"udevadm", "control", "--property=ERROR=0"},
	} {
		if err := testexec.CommandContext(ctx, args[0], args[1:]...).Run(); err != nil {
			s.Fatalf("%s failed: %v", shutil.EscapeSlice(args), err)
		}
	}

	// Check proper Atmel trackpad crash reports are created.
	err = testing.Poll(ctx, func(c context.Context) error {
		found, err := checkAtmelCrashes(pastCrashes)
		if err != nil {
			s.Fatal("Failed while polling crash log: ", err)
		}
		if found {
			return nil
		}
		return errors.New("no Atmel crash found")
	}, &testing.PollOptions{Timeout: 60 * time.Second})
	if err != nil {
		s.Error("Failed to wait for Atmel crash reports: ", err)
	}
}
