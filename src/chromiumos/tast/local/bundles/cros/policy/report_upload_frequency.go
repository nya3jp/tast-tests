// Copyright 2022 The ChromiumOS Authors
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
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ReportUploadFrequency,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Check ReportUploadFrequency by observing /var/log/messages",
		Contacts: []string{
			"zubeil@google.com", // Test author
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      fixture.ChromeEnrolledLoggedIn,
		Timeout:      6 * time.Minute, // Increased timeout as we need to wait for report uploads.
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.ReportUploadFrequency{}, pci.VerifiedFunctionalityOS),
		},
	})
}

func ReportUploadFrequency(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	for _, param := range []struct {
		name          string
		policy        *policy.ReportUploadFrequency
		expectTimeout bool
	}{
		{
			name:          "unset",
			policy:        &policy.ReportUploadFrequency{Stat: policy.StatusUnset},
			expectTimeout: true,
		},
		// TODO(crbug/1286306): If possible, use a lower minimal value to reduce test duration.
		{
			name:          "set60s",
			policy:        &policy.ReportUploadFrequency{Val: 60000}, // 60000 ms is the minimal reporting interval.
			expectTimeout: false,
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Create reader for /var/log/messages.
			reader, err := syslog.NewReader(ctx)
			if err != nil {
				s.Fatal("Failed to initialize syslog reader: ", err)
			}
			defer reader.Close()

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.policy}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			entryMatcher := func(e *syslog.Entry) bool {
				return strings.Contains(e.Content, "Starting status upload: has_device_status = 1")
			}

			// The first upload message will always be issued, even if no policy is set.
			// We need to retrieve two messages to be sure the policy was applied successfully.
			s.Log("Waiting for upload message 1/2 in /var/log/messages")
			if _, err = reader.Wait(ctx, 65*time.Second, entryMatcher); err != nil {
				s.Fatal("Failed to wait for first upload message: ", err)
			}

			s.Log("Waiting for upload message 2/2 in /var/log/messages")
			_, err = reader.Wait(ctx, 65*time.Second, entryMatcher)
			if err != nil && !param.expectTimeout {
				s.Error("Failed to wait for upload message: ", err)
			} else if err == nil && param.expectTimeout {
				s.Error("Received upload message although it should timeout")
			}
		})
	}
}
