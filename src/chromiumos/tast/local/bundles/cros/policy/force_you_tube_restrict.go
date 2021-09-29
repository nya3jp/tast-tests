// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

// ForceYouTubeRestrict policy values.
const (
	forceYouTubeRestrictDisabled = iota
	forceYouTubeRestrictModerate
	forceYouTubeRestrictStrict
)

// There are 3 kinds of contents:
// - Strong content is restricted even for a moderate restriction.
// - Mild content is only restricted when strict restriction is set.
// - Friendly content is never restricted.
const (
	mildContent   = "https://www.youtube.com/watch?v=Fmwfmee2ZTE"
	strongContent = "https://www.youtube.com/watch?v=yR79oLrI1g4"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ForceYouTubeRestrict,
		Desc: "Check if YouTube content restrictions work as specified by the ForceYouTubeRestrict policy",
		Contacts: []string{
			"sinhak@google.com",
			"chromeos-commercial-remote-management@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      fixture.ChromePolicyLoggedIn,
	})
}

// ForceYouTubeRestrict tests the behavior of the ForceYouTubeRestrict Enterprise policy.
func ForceYouTubeRestrict(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	for _, param := range []struct {
		// name is the subtest name.
		name string
		// value is the policy value.
		value *policy.ForceYouTubeRestrict
		// stringContentRestricted is whether the content, which is restricted when moderate or strict restriction is set, is expected to be accessable.
		strongContentRestricted bool
		// mildContentRestricted is whether the content, which is restricted only when strict restriction is set, is expected to be accessable.
		mildContentRestricted bool
	}{
		{
			name:                    "disabled",
			value:                   &policy.ForceYouTubeRestrict{Val: forceYouTubeRestrictDisabled},
			strongContentRestricted: false,
			mildContentRestricted:   false,
		},
		{
			name:                    "moderate",
			value:                   &policy.ForceYouTubeRestrict{Val: forceYouTubeRestrictModerate},
			strongContentRestricted: true,
			mildContentRestricted:   false,
		},
		{
			name:                    "strict",
			value:                   &policy.ForceYouTubeRestrict{Val: forceYouTubeRestrictStrict},
			strongContentRestricted: true,
			mildContentRestricted:   true,
		},
		{
			name:                    "unset",
			value:                   &policy.ForceYouTubeRestrict{Stat: policy.StatusUnset},
			strongContentRestricted: false,
			mildContentRestricted:   false,
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndRefresh(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Run actual test.
			if err := testRestrictedMode(ctx, cr, param.strongContentRestricted, param.mildContentRestricted); err != nil {
				s.Error("Failed to verify YouTube content restriction: ", err)
			}
		})
	}
}

func testRestrictedMode(ctx context.Context, cr *chrome.Chrome, expectedStrongContentRestricted, expectedMildContentRestricted bool) error {
	if mildContentRestricted, err := isYouTubeContentRestricted(ctx, cr, mildContent); err != nil {
		return err
	} else if mildContentRestricted != expectedMildContentRestricted {
		return errors.Errorf("unexpected strictly restricted content accessibility; got %t, wanted %t", mildContentRestricted, expectedMildContentRestricted)
	}

	if strongContentRestricted, err := isYouTubeContentRestricted(ctx, cr, strongContent); err != nil {
		return err
	} else if strongContentRestricted != expectedStrongContentRestricted {
		return errors.Errorf("unexpected moderately restricted content accessibility; got %t, wanted %t", strongContentRestricted, expectedStrongContentRestricted)
	}

	return nil
}

func isYouTubeContentRestricted(ctx context.Context, cr *chrome.Chrome, url string) (bool, error) {
	message, err := getYouTubeErrorMessage(ctx, cr, url)
	if err != nil {
		return false, err
	}

	return message != "", nil
}

// getYouTubeErrorMessage returns the error message, if any, returned by Youtube while trying to view the given url.
func getYouTubeErrorMessage(ctx context.Context, cr *chrome.Chrome, url string) (string, error) {
	conn, err := cr.NewConn(ctx, url)
	if err != nil {
		return "", err
	}
	defer conn.Close()

	var message string
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := conn.Eval(ctx, `document.getElementById('error-screen').innerText`, &message); err != nil {
			return err
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  15 * time.Second,
		Interval: 1 * time.Second,
	}); err != nil {
		return "", err
	}

	return strings.TrimSpace(message), nil
}
