// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/security/seccomp"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         USBBouncer,
		Desc:         "Check that usb_bouncer works as intended",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome", "usbguard"},
		Contacts: []string{
			"allenwebb@chromium.org",
			"jorgelo@chromium.org",
			"chromeos-security@google.com",
		},
	})
}

// pathOfTestDevice finds a path to a USB device in /sys/devices. It returns an empty string on
// failure.
func pathOfTestDevice() (string, error) {
	var devPath string
	if err := filepath.Walk("/sys/devices/", func(path string, info os.FileInfo, err error) error {
		if len(devPath) > 0 {
			return filepath.SkipDir
		}
		if strings.HasSuffix(path, "/authorized") {
			devPath = path[len("/sys"):]
			return filepath.SkipDir
		}
		return nil
	}); err != nil {
		return "", errors.Wrap(err, "unable to get a test device path")
	}
	return devPath, nil
}

func testUsbBouncer(ctx context.Context, s *testing.State, m *seccomp.PolicyGenerator,
	devPath string, withChrome bool) {
	cases := [][]string{
		{"udev", "add", devPath},
		{"cleanup"},
		{"udev", "remove", devPath},
		{"genrules"},
	}

	if withChrome {
		cases = append(cases, []string{"userlogin"})
	}

	for _, c := range cases {
		f, err := ioutil.TempFile(s.OutDir(), "strace-usb_bouncer")
		if err != nil {
			s.Fatal("TempFile failed: ", err)
		}
		f.Close()
		logFile := f.Name()

		cmd := seccomp.CommandContext(ctx, logFile, "usb_bouncer", c...)
		if err := cmd.Run(); err != nil {
			s.Fatalf("%q failed with %v", cmd.Args, err)
		}
		m.AddStraceLog(logFile, seccomp.ExcludeSyscallsBeforeSandboxing)
	}
}

func USBBouncer(ctx context.Context, s *testing.State) {
	const (
		defaultUser   = "testuser@gmail.com"
		defaultPass   = "testpass"
		defaultGaiaID = "gaia-id"
	)

	d, err := pathOfTestDevice()
	if err != nil {
		s.Fatal("Unable to find a suitable test USB device: ", err)
	}
	if len(d) == 0 {
		s.Fatal("Unable to find a suitable test USB device")
	}

	// Move the current state to a temporary location and restore it after the test. This ensures the
	// codepaths that create the state directory and file are exercised.
	globalStateDir := "/run/usb_bouncer"
	stashedGlobalStateDir := "/run/usb_bouncer.orig"
	if err = os.Rename(globalStateDir, stashedGlobalStateDir); err == nil {
		defer func() {
			if err := os.RemoveAll(globalStateDir); err != nil {
				s.Errorf("RemoveAll(%q) failed: %v", globalStateDir, err)
			}
			if err := os.Rename(stashedGlobalStateDir, globalStateDir); err != nil {
				s.Errorf("Rename(%q, %q) failed: %v", stashedGlobalStateDir, globalStateDir, err)
			}
		}()
	} else if !os.IsNotExist(err) {
		s.Fatalf("Failed to stash %q: %v", globalStateDir, err)
	}

	// Clear any usb_bouncer files from the test user's daemon-store. The sub directories need to be
	// preserved.
	userStateDir := "/run/daemon-store/usb_bouncer"
	if err := filepath.Walk(userStateDir, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			return os.Remove(path)
		}
		return nil
	}); err != nil {
		s.Fatalf("Failed to cleanup %q: %v", userStateDir, err)
	}

	m := seccomp.NewPolicyGenerator()
	testUsbBouncer(ctx, s, m, d, false)

	cr, err := chrome.New(ctx, chrome.Auth(defaultUser, defaultPass, defaultGaiaID))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	// Reuse policy generator to accumulate syscalls across both test cases.
	testUsbBouncer(ctx, s, m, d, true)

	policyFile := filepath.Join(s.OutDir(), "usb_bouncer.policy")
	if err := ioutil.WriteFile(policyFile, []byte(m.GeneratePolicy()), 0644); err != nil {
		s.Fatal("Failed to record seccomp policy: ", err)
	}
	s.Logf("Wrote usb_bouncer seccomp policy to %q", policyFile)
}
