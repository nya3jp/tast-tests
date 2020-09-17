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
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/pre"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DisableScreenshots,
		// TODO(crbug:1125556): check whether screenshot can be taken by extensions APIs.
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

	if err := removeScreenshots(); err != nil {
		s.Fatal("Failed to remove all screenshots before running test: ", err)
	}

	// Connect to Test API.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	if err := ash.CloseNotifications(ctx, tconn); err != nil {
		s.Fatal("Failed to close notifications: ", err)
	}

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
			cleanupCtx := ctx
			ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
			defer cancel()

			defer func() {
				if err := removeScreenshots(); err != nil {
					s.Error("Failed to remove screenshots after test: ", err)
				}
			}()

			defer func(ctx context.Context) {
				if err := ash.CloseNotifications(ctx, tconn); err != nil {
					s.Error("Failed to close notifications: ", err)
				}
			}(cleanupCtx)

			defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, tc.value); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			keyboard, err := input.Keyboard(ctx)
			if err != nil {
				s.Fatal("Failed to get keyboard: ", err)
			}

			if err := testing.Poll(ctx, func(ctx context.Context) error {
				// Press keys in the loop, because it takes some time for system to really apply policy.
				if err := keyboard.Accel(ctx, "Ctrl+Scale"); err != nil {
					return testing.PollBreak(errors.Wrap(err, "failed to press keys"))
				}

				if _, err := ash.WaitForNotification(ctx, tconn, 3*time.Second, ash.WaitTitle(tc.wantNotification)); err != nil {
					// Remove screenshots in case if policy was not fully applied and screenshot was taken.
					if err := removeScreenshots(); err != nil {
						return testing.PollBreak(errors.Wrap(err, "failed to remove screenshots"))
					}
					return err
				}
				return nil
			}, &testing.PollOptions{Timeout: 12 * time.Second}); err != nil {
				s.Fatalf("Failed to wait notification with title %q: %v", tc.wantNotification, err)
			}

			paths, err := screenshots()
			if err != nil {
				s.Fatal("Failed to get whether screenshot is present")
			}
			if has := len(paths) > 0; has != tc.wantAllowed {
				s.Fatalf("Unexpected screenshot allowed: get %t; want %t", has, tc.wantAllowed)
			}
		})
	}
}

func screenshots() ([]string, error) {
	re := regexp.MustCompile(`Screenshot.*png`)
	var paths []string

	if err := filepath.Walk("/home/chronos/user/MyFiles/Downloads", func(path string, info os.FileInfo, err error) error {
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
