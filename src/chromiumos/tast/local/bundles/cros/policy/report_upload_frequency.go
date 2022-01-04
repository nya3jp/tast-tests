// Copyright 2022 The Chromium OS Authors. All rights reserved.
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
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ReportUploadFrequency,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Check ReportUploadFrequency by observing /var/log/messages",
		Contacts: []string{
			"zubeil@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      fixture.FakeDMSEnrolled,
		Timeout:      5 * time.Minute, // Increased timeout as we need to wait for two report uploads
	})
}

func ReportUploadFrequency(ctx context.Context, s *testing.State) {
	fdms := s.FixtValue().(*fakedms.FakeDMS)

	// Start a Chrome instance that will fetch policies from the FakeDMS.
	cr, err := chrome.New(ctx,
		chrome.FakeLogin(chrome.Creds{User: fixtures.Username, Pass: fixtures.Password}),
		chrome.DMSPolicy(fdms.URL),
		chrome.KeepEnrollment())
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

	//Set policy
	policies := []policy.Policy{
		&policy.ReportUploadFrequency{
			Val: 60000, //60000 ms is the minimal reporting interval
		},
	}
	if err := policyutil.ServeAndRefresh(ctx, fdms, cr, policies); err != nil {
		s.Fatal("Failed to serve policies: ", err)
	}

	// Create reader for /var/log/messages
	reader, err := syslog.NewReader(ctx)
	if err != nil {
		s.Fatal("Failed to initialize syslog reader: ", err)
	}

	entryMatcher := func(e *syslog.Entry) bool {
		return strings.Contains(e.Content, "Starting status upload: has_device_status = 1")
	}

	// The first upload message will always be issued, even if no policy is set.
	// We need to retrieve two messages to be sure the policy was applied successfully.
	messagesToRetrieve := 2
	for i := 1; i <= messagesToRetrieve; i++ {
		s.Log("Waiting for upload message ", i, "/", messagesToRetrieve, " in /var/log/messages")
		_, err = reader.Wait(ctx, 65*time.Second, entryMatcher)

		if err != nil {
			s.Fatal("Failed to wait for upload message: ", err)
		}
	}
}
