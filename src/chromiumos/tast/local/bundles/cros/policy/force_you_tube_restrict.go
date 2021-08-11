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
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

// ForceYouTubeRestrict policy values.
const (
	forceYouTubeRestrictDisabled = iota
	forceYouTubeRestrictModerate
	forceYouTubeRestrictStrict
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
		Fixture:      "chromePolicyLoggedIn",
	})
}

// ForceYouTubeRestrict tests the behavior of the ForceYouTubeRestrict Enterprise policy.
func ForceYouTubeRestrict(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fdms := s.FixtValue().(*fixtures.FixtData).FakeDMS

	for _, param := range []struct {
		// name is the subtest name.
		name string
		// value is the policy value.
		value *policy.ForceYouTubeRestrict
	}{
		{
			name:  "disabled",
			value: &policy.ForceYouTubeRestrict{Val: forceYouTubeRestrictDisabled},
		},
		{
			name:  "moderate",
			value: &policy.ForceYouTubeRestrict{Val: forceYouTubeRestrictModerate},
		},
		{
			name:  "strict",
			value: &policy.ForceYouTubeRestrict{Val: forceYouTubeRestrictStrict},
		},
		{
			name:  "unset",
			value: &policy.ForceYouTubeRestrict{Stat: policy.StatusUnset},
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
			if err := testRestrictedMode(ctx, cr, param.value); err != nil {
				s.Error("Failed to verify YouTube content restriction: ", err)
			}
		})
	}
}

func testRestrictedMode(ctx context.Context, cr *chrome.Chrome, policyVal *policy.ForceYouTubeRestrict) error {
	var expectedModerateViewability, expectedStrictViewability bool
	if policyVal.Stat == policy.StatusUnset {
		expectedModerateViewability = true
		expectedStrictViewability = true
	} else {
		switch policyVal.Val {
		case forceYouTubeRestrictDisabled:
			expectedModerateViewability = true
			expectedStrictViewability = true
		case forceYouTubeRestrictModerate:
			expectedModerateViewability = false
			expectedStrictViewability = true
		case forceYouTubeRestrictStrict:
			expectedModerateViewability = false
			expectedStrictViewability = false
		default:
			return errors.Errorf("unexpected policy value: %d", policyVal.Val)
		}
	}

	r, err := isStrictlyRestrictedContentViewable(ctx, cr)
	if err != nil {
		return err
	}
	if r != expectedStrictViewability {
		return errors.Errorf("unexpected strictly restricted content accessibility, got %t, wanted %t", r, expectedStrictViewability)
	}

	r, err = isModeratelyRestrictedContentViewable(ctx, cr)
	if err != nil {
		return err
	}
	if r != expectedModerateViewability {
		return errors.Errorf("unexpected moderately restricted content accessibility, got %t, wanted %t", r, expectedModerateViewability)
	}

	return nil
}

func isStrictlyRestrictedContentViewable(ctx context.Context, cr *chrome.Chrome) (bool, error) {
	const strictlyRestrictedVideo = "https://www.youtube.com/watch?v=Fmwfmee2ZTE"
	return isYouTubeContentViewable(ctx, cr, strictlyRestrictedVideo)
}

func isModeratelyRestrictedContentViewable(ctx context.Context, cr *chrome.Chrome) (bool, error) {
	const moderatelyRestrictedVideo = "https://www.youtube.com/watch?v=yR79oLrI1g4"
	return isYouTubeContentViewable(ctx, cr, moderatelyRestrictedVideo)
}

func isYouTubeContentViewable(ctx context.Context, cr *chrome.Chrome, url string) (bool, error) {
	message, err := getYouTubeErrorMessage(ctx, cr, url)
	if err != nil {
		return false, err
	}

	return message == "", nil
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
