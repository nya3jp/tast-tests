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
	"chromiumos/tast/local/policyutil/pre"
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
		Desc: "Behavior of ForceYouTubeRestrict policy",
		Contacts: []string{
			"sinhak@google.com",
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          pre.User,
	})
}

// ForceYouTubeRestrict tests the behavior of the ForceYouTubeRestrict Enterprise policy.
func ForceYouTubeRestrict(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*pre.PreData).Chrome
	fdms := s.PreValue().(*pre.PreData).FakeDMS

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
				s.Error("Something failed: ", err)
			}
		})
	}
}

func testRestrictedMode(ctx context.Context, cr *chrome.Chrome, policyVal *policy.ForceYouTubeRestrict) error {
	if policyVal.Stat == policy.StatusUnset {
		return testRestrictedModeDisabled(ctx, cr)
	}

	switch policyVal.Val {
	case forceYouTubeRestrictDisabled:
		return testRestrictedModeDisabled(ctx, cr)
	case forceYouTubeRestrictModerate:
		return testRestrictedModeModerate(ctx, cr)
	case forceYouTubeRestrictStrict:
		return testRestrictedModeStrict(ctx, cr)
	}

	return nil
}

func testRestrictedModeDisabled(ctx context.Context, cr *chrome.Chrome) error {
	r, err := isStrictlyRestrictedContentViewable(ctx, cr)
	if err != nil {
		return err
	}
	if !r {
		return errors.New("Strictly restricted content is not viewable")
	}

	r, err = isModeratelyRestrictedContentViewable(ctx, cr)
	if err != nil {
		return err
	}
	if !r {
		return errors.New("Moderately restricted content is not viewable")
	}

	return nil
}

func testRestrictedModeModerate(ctx context.Context, cr *chrome.Chrome) error {
	r, err := isStrictlyRestrictedContentViewable(ctx, cr)
	if err != nil {
		return err
	}
	if !r {
		return errors.New("Strictly restricted content is not viewable")
	}

	r, err = isModeratelyRestrictedContentViewable(ctx, cr)
	if err != nil {
		return err
	}
	if r {
		return errors.New("Moderately restricted content is viewable")
	}

	return nil
}

func testRestrictedModeStrict(ctx context.Context, cr *chrome.Chrome) error {
	r, err := isStrictlyRestrictedContentViewable(ctx, cr)
	if err != nil {
		return err
	}
	if r {
		return errors.New("Strictly restricted content is viewable")
	}

	r, err = isModeratelyRestrictedContentViewable(ctx, cr)
	if err != nil {
		return err
	}
	if r {
		return errors.New("Moderately restricted content is viewable")
	}

	return nil
}

func isStrictlyRestrictedContentViewable(ctx context.Context, cr *chrome.Chrome) (bool, error) {
	const strictlyRestrictedVideo = "https://www.youtube.com/watch?v=Fmwfmee2ZTE"
	return isContentViewable(ctx, cr, strictlyRestrictedVideo)
}

func isModeratelyRestrictedContentViewable(ctx context.Context, cr *chrome.Chrome) (bool, error) {
	const moderatelyRestrictedVideo = "https://www.youtube.com/watch?v=yR79oLrI1g4"
	return isContentViewable(ctx, cr, moderatelyRestrictedVideo)
}

func isContentViewable(ctx context.Context, cr *chrome.Chrome, url string) (bool, error) {
	message, err := getErrorMessage(ctx, cr, url)
	if err != nil {
		return false, err
	}

	return message == "", nil
}

// getErrorMessage returns the error message, if any, returned by Youtube while trying to view the given url.
func getErrorMessage(ctx context.Context, cr *chrome.Chrome, url string) (string, error) {
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
		Timeout:  5 * time.Second,
		Interval: 1 * time.Second,
	}); err != nil {
		return "", err
	}

	return strings.TrimSpace(message), nil
}
