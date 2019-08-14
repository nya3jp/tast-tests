// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/strace"
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

func getTestDevicePath() string {
	devPath := ""
	filepath.Walk("/sys/devices/", func(path string, info os.FileInfo, err error) error {
		if len(devPath) > 0 {
			return filepath.SkipDir
		}
		if strings.HasSuffix(path, "/authorized") {
			devPath = path[len("/sys"):]
			return filepath.SkipDir
		}
		return nil
	})
	return devPath
}

func usbBouncerTests(ctx context.Context, s *testing.State, m *strace.MinijailPolicyGenerator,
	devPath string, withChrome bool) {
	type testCase struct {
	}
	cases := [][]string{
		{"udev", "add", devPath},
		{"cleanup"},
		{"udev", "remove", devPath},
		{"genrules"},
	}

	if withChrome {
		cases = append(cases, []string{"userlogin"})
	}

	os.Remove("/run/usb_bouncer")
	filepath.Walk("/run/daemon-store/usb_bouncer", func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			os.Remove(path)
		}
		return nil
	})

	for _, c := range cases {
		cmd := strace.CommandContext(ctx, "usb_bouncer", c...)
		if err := cmd.Run(); err != nil {
			s.Fatalf("%q failed with %v", cmd.Args, err)
		}
		cmd.ApplyResultToPolicyGenerator(m, strace.ExcludeSyscallsBeforeSandboxing)
	}
}

func USBBouncer(ctx context.Context, s *testing.State) {
	const (
		defaultUser   = "testuser@gmail.com"
		defaultPass   = "testpass"
		defaultGaiaID = "gaia-id"
	)

	d := getTestDevicePath()
	if len(d) == 0 {
		s.Fatal("Unable to find a sutable test USB device")
		return
	}

	m := strace.NewMinijailPolicyGenerator()
	usbBouncerTests(ctx, s, m, d, false)

	cr, err := chrome.New(ctx, chrome.ExtraArgs(), chrome.Auth(defaultUser, defaultPass, defaultGaiaID))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	usbBouncerTests(ctx, s, m, d, true)

	s.Log(m.GeneratePolicy())
}
