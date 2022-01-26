// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package mgs

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/mgs"
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
		Func:         ForceYouTubeRestrict,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Verify behavior of ForceYouTubeRestrict policy on Managed Guest Session",
		Contacts: []string{
			"cmfcmf@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      fixture.FakeDMSEnrolled,
	})
}

// ForceYouTubeRestrict tests the behavior of the ForceYouTubeRestrict Enterprise policy.
func ForceYouTubeRestrict(ctx context.Context, s *testing.State) {
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	for _, param := range []struct {
		// name is the subtest name.
		name string
		// value is the policy value.
		value *policy.ForceYouTubeRestrict
		// stringContentRestricted is whether strong content is expected to be restricted.
		strongContentRestricted bool
		// mildContentRestricted is whether mild content is expected to be restricted.
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
			// Launch a new MGS with default account.
			mgs, cr, err := mgs.New(
				ctx,
				fdms,
				mgs.DefaultAccount(),
				mgs.AutoLaunch(mgs.MgsAccountID),
				mgs.AddPublicAccountPolicies(mgs.MgsAccountID, []policy.Policy{param.value}),
			)
			if err != nil {
				s.Fatal("Failed to start Chrome on Signin screen with default MGS account: ", err)
			}
			defer func() {
				if err := mgs.Close(ctx); err != nil {
					s.Fatal("Failed close MGS: ", err)
				}
			}()
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_")
			br := cr.Browser()

			// Run actual test.
			if err := testRestrictedMode(ctx, br, param.strongContentRestricted, param.mildContentRestricted); err != nil {
				s.Error("Failed to verify YouTube content restriction: ", err)
			}
		})
	}
}

func testRestrictedMode(ctx context.Context, br ash.ConnSource, expectedStrongContentRestricted, expectedMildContentRestricted bool) error {
	if mildContentRestricted, err := isYouTubeContentRestricted(ctx, br, mildContent); err != nil {
		return err
	} else if mildContentRestricted != expectedMildContentRestricted {
		return errors.Errorf("unexpected mild content restriction; got %t, wanted %t", mildContentRestricted, expectedMildContentRestricted)
	}

	if strongContentRestricted, err := isYouTubeContentRestricted(ctx, br, strongContent); err != nil {
		return err
	} else if strongContentRestricted != expectedStrongContentRestricted {
		return errors.Errorf("unexpected strong content restriction; got %t, wanted %t", strongContentRestricted, expectedStrongContentRestricted)
	}

	return nil
}

func isYouTubeContentRestricted(ctx context.Context, br ash.ConnSource, url string) (bool, error) {
	message, err := getYouTubeErrorMessage(ctx, br, url)
	if err != nil {
		return false, err
	}

	return message != "", nil
}

// getYouTubeErrorMessage returns the error message, if any, returned by Youtube while trying to view the given url.
func getYouTubeErrorMessage(ctx context.Context, br ash.ConnSource, url string) (string, error) {
	conn, err := br.NewConn(ctx, url)
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
