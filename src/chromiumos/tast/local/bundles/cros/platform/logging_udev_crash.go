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
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/testing"
)

const whitelistDir = "/var/lib/whitelist"
const onwerKeyFile = whitelistDir + "/owner.key"
const signedPolicyFile = whitelistDir + "/policy"
const systemCrashDir = "/var/spool/crash"
const mockMetricsOnPolicyFile = "logging_udev_crash_mock_metrics_on.policy"
const mockMetricsOffPolicyFile = "logging_udev_crash_mock_metrics_off.policy"
const mockMetricsOwnerKeyFile = "logging_udev_crash_mock_metrics_owner.key"

func init() {
	testing.AddTest(&testing.Test{
		Func:     LoggingUdevCrash,
		Desc:     "Verify udev triggered crash works as expected",
		Contacts: []string{"yamaguchi@chromium.org", "tast-users@chromium.org"},
		Attr:     []string{"informational"},
		Data: []string{
			mockMetricsOnPolicyFile,
			mockMetricsOffPolicyFile,
			mockMetricsOwnerKeyFile,
		},
	})
}

func openWriteClose(name string, content string) error {
	file, err := os.Create(name)
	if err != nil {
		return errors.Errorf("failed to create %s", name)
	}
	defer file.Close()
	file.Write(([]byte)(content))
	return nil
}

func isDir(name string) bool {
	if e, err := os.Stat(whitelistDir); err == nil {
		return e.IsDir()
	}
	return false
}

// setConsent sets whether or not we have consent to send crash reports.
// This creates or deletes the |consentFile| to control whether
// crash_sender will consider that it has consent to send crash reports.
// It also copies a policy blob with the proper policy setting.
func setConsent(hasConsent bool, s *testing.State) error {
	const consentFile = "/home/chronos/Consent To Send Stats"
	if hasConsent {
		if isDir(whitelistDir) {
			// Create policy file that enables metrics/consent.
			if err := fsutil.CopyFile(filepath.Join(s.DataPath(mockMetricsOnPolicyFile)),
				signedPolicyFile); err != nil {
				return err
			}
			if err := fsutil.CopyFile(filepath.Join(s.DataPath(mockMetricsOwnerKeyFile)),
				onwerKeyFile); err != nil {
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
		openWriteClose(tempFile, "test-consent")
		if err := exec.Command("chown", "chronos:chronos", tempFile).Run(); err != nil {
			return err
		}
		if err := os.Rename(tempFile, consentFile); err != nil {
			return err
		}
		s.Logf("Created %s", consentFile)
	} else {
		if isDir(whitelistDir) {
			// Create policy file that disables metrics/consent.
			if err := fsutil.CopyFile(filepath.Join(s.DataPath(mockMetricsOffPolicyFile)),
				signedPolicyFile); err != nil {
				return err
			}
			if err := fsutil.CopyFile(filepath.Join(s.DataPath(mockMetricsOwnerKeyFile)),
				onwerKeyFile); err != nil {
				return err
			}
		}
		// Remove deprecated consent file.
		os.Remove(consentFile)
	}
	return nil
}

func checkAtmelCrashes(s *testing.State) bool {
	// Check proper Atmel trackpad crash reports are created.
	if _, err := os.Stat(systemCrashDir); err != nil {
		s.Errorf("cannot stat system crash dir: %s", systemCrashDir)
		return false
	}

	files, err := ioutil.ReadDir(systemCrashDir)
	if err != nil {
		s.Fatal("failed to read Atmel dir")
	}
	for _, file := range files {
		filename := file.Name()
		path := filepath.Join(systemCrashDir, filename)
		if !strings.HasPrefix(filename, "change__i2c_atmel_mxt_ts") {
			s.Fatalf("Crash report %s has wrong name", filename)
		}
		if strings.HasSuffix(filename, "meta") {
			continue
		}

		var r io.Reader
		if strings.HasSuffix(filename, ".log.gz") {
			archive, _ := os.Open(path)
			r, _ = gzip.NewReader(archive)
		} else if strings.HasSuffix(filename, ".log") {
			r, _ = os.Open(path)
		} else {
			s.Fatalf("Crash report %s has wrong extension", filename)
		}

		lines, _ := ioutil.ReadAll(r)
		// Check that we have seen the end of the file. Otherwise we could
		// end up racing bwtween writing to the log file and reading/checking
		// the log file.
		if !strings.Contains(string(lines), "END-OF-LOG") {
			continue
		}

		badLines := []string{}
		for _, line := range strings.Split(string(lines), "\n") {
			if len(line) > 0 && !strings.Contains(line, "atmel_mxt_ts") && !strings.Contains(line, "END-OF-LOG") {
				badLines = append(badLines, line)
			}
		}
		if len(badLines) > 0 {
			s.Fatalf("Crash report contains invalid content %s", badLines)
		}
		return true
	}
	return false
}

func LoggingUdevCrash(ctx context.Context, s *testing.State) {
	const driverDir = "/sys/bus/i2c/drivers/atmel_mxt_ts"
	hasAtmelDevice := false

	if _, err := os.Stat(driverDir); err == nil {
		files, err := ioutil.ReadDir(driverDir)
		if err != nil {
			s.Fatal("failed to read Atmel dir")
		}
		for _, file := range files {
			if file.Mode()&os.ModeSymlink != 0 {
				fullpath, _ := filepath.EvalSymlinks(filepath.Join(driverDir, file.Name()))
				file, _ = os.Stat(fullpath)
			}
			if file.Mode().IsDir() {
				hasAtmelDevice = true
			}
		}
	}
	if !hasAtmelDevice {
		s.Log("No atmel device, skip the test")
		return
	}

	if err := setConsent(true, s); err != nil {
		s.Fatal("failed to set consent: ", err)
	}

	// Use udevadm to trigger a fake udev event representing atmel driver
	// failure. The uevent match rule in 99-crash-reporter.rules is
	// ACTION=="change", SUBSYSTEM=="i2c", DRIVER=="atmel_mxt_ts",
	// ENV{ERROR}=="1" RUN+="/sbin/crash_reporter
	// --udev=SUBSYSTEM=i2c-atmel_mxt_ts:ACTION=change"

	exec.Command("udevadm", "control", "--property=ERROR=1").Run()
	exec.Command("udevadm", "trigger",
		"--action=change",
		"--subsystem-match=i2c",
		"--attr-match=driver=atmel_mxt_ts").Run()
	exec.Command("udevadm", "control", "--property=ERROR=0").Run()

	result := testing.Poll(ctx, func(c context.Context) error {
		if checkAtmelCrashes(s) {
			return nil
		}
		return errors.New("")
	}, &testing.PollOptions{Timeout: 60 * time.Second})
	if result != nil {
		s.Error("No valid Atmel crash reports")
	}
}
