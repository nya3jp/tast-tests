// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/pre"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: KeyboardDefaultToFunctionKeys,
		Desc: "Behavior of the KeyboardDefaultToFunctionKeys policy",
		Contacts: []string{
			"mpolzer@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          pre.User,
	})
}

// KeyboardDefaultToFunctionKeys tests default function key action.
// Search+function keys should perform the alternate action.
func KeyboardDefaultToFunctionKeys(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*pre.PreData).Chrome
	fdms := s.PreValue().(*pre.PreData).FakeDMS
	for _, tc := range []struct {
		name   string
		value  *policy.KeyboardDefaultToFunctionKeys
		key    string // The key(s) to be pressed
		expect string // What we expect to find after the pressed key
	}{
		{
			name:   "true + explore",
			value:  &policy.KeyboardDefaultToFunctionKeys{Val: true},
			key:    "f1",
			expect: "Explore",
		},
		{
			name:   "false + explore",
			value:  &policy.KeyboardDefaultToFunctionKeys{Val: false},
			key:    "search+f1",
			expect: "Explore",
		},
		{
			name:   "notset + explore",
			value:  &policy.KeyboardDefaultToFunctionKeys{Stat: policy.StatusUnset},
			key:    "search+f1",
			expect: "Explore",
		},
		{
			name:   "true + back",
			value:  &policy.KeyboardDefaultToFunctionKeys{Val: true},
			key:    "search+f1",
			expect: "chrome://settings/",
		},
		{
			name:   "false + back",
			value:  &policy.KeyboardDefaultToFunctionKeys{Val: false},
			key:    "f1",
			expect: "chrome://settings/",
		},
		{
			name:   "notset + back",
			value:  &policy.KeyboardDefaultToFunctionKeys{Stat: policy.StatusUnset},
			key:    "f1",
			expect: "chrome://settings/",
		},
	} {
		s.Run(ctx, tc.name, func(ctx context.Context, s *testing.State) {
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{tc.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			conn, err := cr.NewConn(ctx, "chrome://settings")
			if err != nil {
				s.Fatal("Failed to connect to chrome: ", err)
			}
			defer conn.Close()

			if err := conn.Navigate(ctx, "chrome://dino"); err != nil {
				s.Fatal("Failed to navigate: ", err)
			}

			kb, err := input.Keyboard(ctx)
			if err != nil {
				s.Fatal("Failed to get keyboard: ", err)
			}
			defer kb.Close()

			kb.Accel(ctx, tc.key)

			if tc.expect == "Explore" {
				// Check that the explore window was opened.
				tconn, err := cr.TestAPIConn(ctx)
				if err != nil {
					s.Fatal("Failed to get TestConn: ", err)
				}
				if err := testing.Poll(ctx, func(ctx context.Context) error {
					windows, err := ash.GetAllWindows(ctx, tconn)
					if err != nil {
						return testing.PollBreak(errors.Wrap(err, "failed to get windows"))
					}

					for _, window := range windows {
						if strings.Contains(window.Title, tc.expect) {
							return nil
						}
					}
					return errors.New("failed to find expected window title")
				}, &testing.PollOptions{
					Timeout: 15 * time.Second,
				}); err != nil {
					s.Errorf("Failed to find explore window: %s", err)
				}
			} else {
				// Check that Chrome navigated back to the settings page.
				if err := testing.Poll(ctx, func(ctx context.Context) error {
					var url string
					if err := conn.Eval(ctx, `document.URL`, &url); err != nil {
						return testing.PollBreak(errors.Wrap(err, "could not read URL"))
					}

					if url == tc.expect {
						return nil
					}
					return errors.Errorf("found %s, expected %s", url, tc.expect)
				}, &testing.PollOptions{
					Timeout: 15 * time.Second,
				}); err != nil {
					s.Errorf("Failed waiting for page: %s", err)
				}
			}
		})
	}
}
