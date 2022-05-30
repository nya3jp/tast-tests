// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package screenshare contains functionality shared by tests that
// exercise DLP screenshare restrictions.
package screenshare

import (
	"context"
	"time"

	"go.chromium.org/chromiumos/tast-tests/common/policy"
	"go.chromium.org/chromiumos/tast/errors"
	"go.chromium.org/chromiumos/tast-tests/local/bundles/cros/dlp/restrictionlevel"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/browser"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/uiauto"
	"go.chromium.org/chromiumos/tast/testing"
)

// Screen share notification strings.
const (
	ScreensharePausedTitle       = "Screen share paused"
	ScreensharePausedIDContains  = "screen_share_dlp_paused-"
	ScreenshareResumedTitle      = "Screen share resumed"
	ScreenshareResumedIDContains = "screen_share_dlp_resumed-"
)

// Different paths used for testing.
const (
	UnrestrictedPath = "/text_1.html"
	RestrictedPath   = "/text_2.html"
)

// The TestParams struct contains parameters for different screenshare tests.
type TestParams struct {
	Name        string
	Restriction restrictionlevel.RestrictionLevel
	Path        string
	BrowserType browser.Type
}

// GetScreenshareBlockPolicy returns a DLP policy that blocks screen sharing.
func GetScreenshareBlockPolicy(serverURL string) []policy.Policy {
	return []policy.Policy{&policy.DataLeakPreventionRulesList{
		Val: []*policy.DataLeakPreventionRulesListValue{
			{
				Name:        "Disable sharing the screen with confidential content visible",
				Description: "User should not be able to share the screen with confidential content visible",
				Sources: &policy.DataLeakPreventionRulesListValueSources{
					Urls: []string{
						serverURL + RestrictedPath,
					},
				},
				Restrictions: []*policy.DataLeakPreventionRulesListValueRestrictions{
					{
						Class: "SCREEN_SHARE",
						Level: "BLOCK",
					},
				},
			},
		},
	},
	}
}

// GetScreenshareWarnPolicy returns a DLP policy that warns when trying to share the screen.
func GetScreenshareWarnPolicy(serverURL string) []policy.Policy {
	return []policy.Policy{&policy.DataLeakPreventionRulesList{
		Val: []*policy.DataLeakPreventionRulesListValue{
			{
				Name:        "Warn before sharing the screen with confidential content visible",
				Description: "User should be warned before sharing the screen with confidential content visible",
				Sources: &policy.DataLeakPreventionRulesListValueSources{
					Urls: []string{
						serverURL + RestrictedPath,
					},
				},
				Restrictions: []*policy.DataLeakPreventionRulesListValueRestrictions{
					{
						Class: "SCREEN_SHARE",
						Level: "WARN",
					},
				},
			},
		},
	},
	}
}

// CheckFrameStatus checks the screen recorder frame status and returns an error if it is different than expected.
func CheckFrameStatus(ctx context.Context, screenRecorder *uiauto.ScreenRecorder, wantAllowed bool) error {
	if screenRecorder == nil {
		return errors.New("couldn't check frame status. Screen recorder was not found")
	}

	// Checking the frame status can randomly fail, so we poll instead.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		status, err := screenRecorder.FrameStatus(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get status of frame")
		}

		if status != "Success" && wantAllowed {
			return errors.Errorf("Frame not recording. got: %v, want Success", status)
		}

		if status != "Fail" && !wantAllowed {
			return errors.Errorf("Frame recording. got: %v, want Fail", status)
		}

		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second, Interval: 500 * time.Millisecond}); err != nil {
		return errors.Wrap(err, "polling the frame status timed out")
	}

	return nil
}
