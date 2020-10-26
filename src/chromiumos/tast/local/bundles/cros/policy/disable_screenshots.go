// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/pre"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DisableScreenshots,
		// TODO(crbug.com/1125556): check whether screenshot can be taken by extensions APIs.
		Desc: "Behavior of the DisableScreenshots policy, check whether screenshot can be taken by pressing hotkeys",
		Contacts: []string{
			"lamzin@google.com", // Test port author
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          pre.User,
	})
}

func DisableScreenshots(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*pre.PreData).Chrome
	fdms := s.PreValue().(*pre.PreData).FakeDMS

	defer func() {
		if err := removeScreenshots(); err != nil {
			s.Error("Failed to remove screenshots after all tests: ", err)
		}
	}()

	// Connect to Test API.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	keyboard, err := input.VirtualKeyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer keyboard.Close()

	for _, tc := range []struct {
		name             string
		value            []policy.Policy
		wantAllowed      bool
		wantNotification string
	}{
		{
			name:             "true",
			value:            []policy.Policy{&policy.DisableScreenshots{Val: true}},
			wantAllowed:      false,
			wantNotification: "Screenshots disabled",
		},
		{
			name:             "false",
			value:            []policy.Policy{&policy.DisableScreenshots{Val: false}},
			wantAllowed:      true,
			wantNotification: "Screenshot taken",
		},
		{
			name:             "unset",
			value:            []policy.Policy{},
			wantAllowed:      true,
			wantNotification: "Screenshot taken",
		},
	} {
		s.Run(ctx, tc.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

			// Minimum interval between screenshot commands is 1 second, so we
			// must sleep for 1 seconds to be able to take screenshot,
			// otherwise hotkey pressing will be ignored.
			//
			// Please check kScreenshotMinimumIntervalInMS constant in
			// ui/snapshot/screenshot_grabber.cc
			testing.Sleep(ctx, time.Second)

			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			if err := ash.CloseNotifications(ctx, tconn); err != nil {
				s.Fatal("Failed to close notifications: ", err)
			}

			if err := removeScreenshots(); err != nil {
				s.Fatal("Failed to remove screenshots: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, tc.value); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			if err := keyboard.Accel(ctx, "Ctrl+F5"); err != nil {
				s.Fatal("Failed to press Ctrl+F5 to take screenshot: ", err)
			}

			if _, err := ash.WaitForNotification(ctx, tconn, 15*time.Second, ash.WaitIDContains("screenshot"), ash.WaitTitle(tc.wantNotification)); err != nil {
				s.Fatalf("Failed to wait notification with title %q: %v", tc.wantNotification, err)
			}

			paths, err := screenshots()
			if err != nil {
				s.Fatal("Failed to check whether screenshot is present")
			}
			if has := len(paths) > 0; has != tc.wantAllowed {
				s.Errorf("Unexpected screenshot allowed: get %t; want %t", has, tc.wantAllowed)
			}
		})
	}
}

func screenshots() ([]string, error) {
	re := regexp.MustCompile(`Screenshot.*png`)
	var paths []string

	if err := filepath.Walk(filesapp.DownloadPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return errors.Wrap(err, "failed to walk through files in Downloads folder")
		}
		if re.FindString(info.Name()) != "" {
			paths = append(paths, path)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return paths, nil
}

func removeScreenshots() error {
	paths, err := screenshots()
	if err != nil {
		return errors.Wrap(err, "failed to get list of screenshots")
	}

	for _, path := range paths {
		if err := os.Remove(path); err != nil {
			return errors.Wrapf(err, "failed to remove %q file", path)
		}
	}

	return nil
}
