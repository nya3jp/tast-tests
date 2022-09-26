// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/pci"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         KeyboardDefaultToFunctionKeys,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Test the KeyboardDefaultToFunctionKeys policy: check that the behavior of function keys is swapped",
		Contacts: []string{
			"mpolzer@google.com", // Test author
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:commercial_limited"},
		Fixture:      fixture.ChromePolicyLoggedIn,
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.KeyboardDefaultToFunctionKeys{}, pci.VerifiedFunctionalityJS),
		},
	})
}

// KeyboardDefaultToFunctionKeys tests default function key action.
// Search+function keys should perform the alternate action.
func KeyboardDefaultToFunctionKeys(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	// Old ChromeOS keyboards send F1 and software maps it to "back".
	// Wilco devices and newer ChromeOS keyboards directly send "back".
	topRow, err := input.KeyboardTopRowLayout(ctx, kb)
	if err != nil {
		s.Fatal("Failed to obtain kayobard layout: ", err)
	}

	back := "F1"
	if topRow.BrowserBack != "F1" {
		back = "back"
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get TestConn: ", err)
	}

	for _, tc := range []struct {
		name  string
		value *policy.KeyboardDefaultToFunctionKeys
		keys  []string // The two test keys. The first one should trigger "back" and the second one should trigger "F1".
	}{
		{
			name:  "true",
			value: &policy.KeyboardDefaultToFunctionKeys{Val: true},
			keys:  []string{"search+" + back, back},
		},
		{
			name:  "false",
			value: &policy.KeyboardDefaultToFunctionKeys{Val: false},
			keys:  []string{back, "search+" + back},
		},
		{
			name:  "unset",
			value: &policy.KeyboardDefaultToFunctionKeys{Stat: policy.StatusUnset},
			keys:  []string{back, "search+" + back},
		},
	} {
		s.Run(ctx, tc.name, func(ctx context.Context, s *testing.State) {
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{tc.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			settings := "chrome://settings/"
			conn, err := cr.NewConn(ctx, settings)
			if err != nil {
				s.Fatal("Failed to connect to chrome: ", err)
			}
			defer conn.Close()

			if err := conn.Navigate(ctx, "chrome://dino"); err != nil {
				s.Fatal("Failed to navigate: ", err)
			}

			// Press the key(s) that should trigger "back".
			if err != kb.Accel(ctx, tc.keys[0]) {
				s.Fatalf("Typing %q failed: %v", tc.keys[0], err)
			}

			// Check that Chrome navigated back to the settings page.
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				var url string
				if err := conn.Eval(ctx, `document.URL`, &url); err != nil {
					return testing.PollBreak(errors.Wrap(err, "could not read URL"))
				}

				if url == settings {
					return nil
				}
				return errors.Errorf("unexpected URL; got %q, want %q", url, settings)
			}, &testing.PollOptions{
				Timeout: 15 * time.Second,
			}); err != nil {
				s.Error("Failed waiting for page: ", err)
			}

			// Press the key(s) that should trigger "F1" and open the "Explore" window.
			if err != kb.Accel(ctx, tc.keys[1]) {
				s.Fatalf("Typing %q failed: %v", tc.keys[1], err)
			}

			// Check that the "Explore" window was opened.
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				windows, err := ash.GetAllWindows(ctx, tconn)
				if err != nil {
					return testing.PollBreak(errors.Wrap(err, "failed to get windows"))
				}

				for _, window := range windows {
					if strings.Contains(window.Title, "Explore") {
						return nil
					}
				}
				return errors.New("failed to find expected window title")
			}, &testing.PollOptions{
				Timeout: 15 * time.Second,
			}); err != nil {
				s.Error("Failed to find explore window: ", err)
			}
		})
	}
}
