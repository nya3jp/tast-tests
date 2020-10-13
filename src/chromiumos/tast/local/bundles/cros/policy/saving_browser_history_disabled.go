// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/pre"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SavingBrowserHistoryDisabled,
		Desc: "Behavior of SavingBrowserHistoryDisabled policy, check if browsing history entries are shown based on the value of the policy",
		Contacts: []string{
			"alexanderhartl@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          pre.User,
	})
}

func SavingBrowserHistoryDisabled(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*pre.PreData).Chrome
	fdms := s.PreValue().(*pre.PreData).FakeDMS

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Create a server that serves an empty html document.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/html")
		io.WriteString(w, "")
	}))
	defer server.Close()

	for _, param := range []struct {
		name    string
		enabled bool                                 // enabled is the expected enabled state of the browsing history.
		policy  *policy.SavingBrowserHistoryDisabled // policy is the policy we test.
	}{
		{
			name:    "unset",
			enabled: true,
			policy:  &policy.SavingBrowserHistoryDisabled{Stat: policy.StatusUnset},
		},
		{
			name:    "history_disabled",
			enabled: false,
			policy:  &policy.SavingBrowserHistoryDisabled{Val: true},
		},
		{
			name:    "history_enabled",
			enabled: true,
			policy:  &policy.SavingBrowserHistoryDisabled{Val: false},
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

			// Clear the browser history.
			if err := tconn.Eval(ctx, `tast.promisify(chrome.browsingData.removeHistory({"since": 0}))`, nil); err != nil {
				s.Fatal("Failed to clear browsing history: ", err)
			}

			// Open a website to create a browsing history entry.
			conn, err := cr.NewConn(ctx, server.URL)
			if err != nil {
				s.Fatal("Failed to connect to chrome: ", err)
			}
			defer conn.Close()

			hconn, err := cr.NewConn(ctx, "chrome://history")
			if err != nil {
				s.Fatal("Failed to connect to chrome: ", err)
			}
			defer hconn.Close()

			// Check whether there is a browser history or not.
			histFound := false
			if err := testing.Poll(ctx, func(ctx context.Context) error {

				// Check if there is a browser history entry.
				if exist, err := ui.Exists(ctx, tconn, ui.FindParams{
					Role:      ui.RoleTypeLink,
					ClassName: "website-link",
				}); err != nil && !errors.Is(err, ui.ErrNodeDoesNotExist) {
					return testing.PollBreak(errors.Wrap(err, "finding website-link node failed"))
				} else if exist {
					histFound = true
					return nil
				}

				// Check if there is no browser history.
				if exist, err := ui.Exists(ctx, tconn, ui.FindParams{
					Role: ui.RoleTypeStaticText,
					Name: "Your browsing history appears here",
				}); err != nil && !errors.Is(err, ui.ErrNodeDoesNotExist) {
					return testing.PollBreak(errors.Wrap(err, "finding text node failed"))
				} else if exist {
					histFound = false
					return nil
				}

				return errors.New("requested ui nodes not found")

			}, &testing.PollOptions{Timeout: 20 * time.Second}); err != nil {
				s.Fatal("Failed to check if history exists: ", err)
			}

			if histFound != param.enabled {
				s.Errorf("Unexpected existence of browser history found: got %t; want %t", histFound, param.enabled)
			}
		})
	}
}
