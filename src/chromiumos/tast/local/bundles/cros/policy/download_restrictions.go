// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/policy/pre"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DownloadRestrictions,
		Desc: "Behavior of DownloadRestrictions policy, check if a file is downloaded or not based on how the policy is set",
		Contacts: []string{
			"alexanderhartl@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          pre.User,
		Data:         []string{"download_restrictions_index.html", "download_restrictions.zip"},
	})
}

func DownloadRestrictions(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*pre.PreData).Chrome
	fdms := s.PreValue().(*pre.PreData).FakeDMS

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	filename := "download_restrictions.zip"

	for _, param := range []struct {
		name    string
		blocked bool
		policy  *policy.DownloadRestrictions // policy is the policy we test.
	}{
		{
			name:    "unset",
			blocked: false,
			policy:  &policy.DownloadRestrictions{Stat: policy.StatusUnset},
		},
		{
			name:    "block_downloads",
			blocked: true,
			policy:  &policy.DownloadRestrictions{Val: 3}, // 3: all downloads are blocked
		},
		{
			name:    "allow_downloads",
			blocked: false,
			policy:  &policy.DownloadRestrictions{Val: 0}, // 0: all downloads are allowed
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.policy}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			dconn, err := cr.NewConn(ctx, server.URL+"/download_restrictions_index.html")
			if err != nil {
				s.Fatal("Failed to connect to chrome: ", err)
			}
			defer dconn.Close()

			err = dconn.Exec(ctx, `document.getElementById('dlink').click()`)
			if err != nil {
				s.Fatal("Failed to execute JS expression: ", err)
			}

			conn, err := cr.NewConn(ctx, "chrome://downloads")
			if err != nil {
				s.Fatal("Failed to connect to chrome: ", err)
			}
			defer conn.Close()

			var pollOptions testing.PollOptions
			if param.blocked {
				pollOptions.Timeout = 5 * time.Second
			}

			if err := testing.Poll(ctx, func(ctx context.Context) error {

				var message string
				// Get the first element in the download history of Chrome.
				err = conn.Eval(ctx, `document.querySelector("body > downloads-manager").shadowRoot.querySelector("#frb0").shadowRoot.querySelector("#file-link").innerText`, &message)
				if err != nil {
					return errors.Wrap(err, "failed to evaluate JS expression")
				} else if !strings.Contains(message, filename) {
					return errors.New("download_restrictions.zip was not downloaded")
				}

				return nil

			}, &pollOptions); err != nil {
				if !param.blocked {
					s.Error("Download was blocked: ", err)
				}
			} else {
				if param.blocked {
					s.Error("Download was not blocked")
				}

				if filename == "download_restrictions.zip" {
					filename = "download_restrictions (1).zip"
				} else {
					filename = "download_restrictions (2).zip"
				}

				// Delete the first element in the download history of Chrome.
				err = conn.Exec(ctx, `document.querySelector("body > downloads-manager").shadowRoot.querySelector("#frb0").shadowRoot.querySelector("#remove").click()`)
				if err != nil {
					s.Fatal("Failed to execute JS expression: ", err)
				}
			}

		})
	}
}
