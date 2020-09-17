// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/ui"
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
		Data:         []string{"saving_browser_history_disabled.html"},
	})
}

func SavingBrowserHistoryDisabled(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*pre.PreData).Chrome
	fdms := s.PreValue().(*pre.PreData).FakeDMS

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
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
			chconn, err := cr.NewConn(ctx, "chrome://settings/clearBrowserData")
			if err != nil {
				s.Fatal("Failed to connect to chrome: ", err)
			}
			defer chconn.Close()

			// Find the Time range PopUpButton node and click it.
			trNode, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{
				Role: ui.RoleTypePopUpButton,
				Name: "Time range",
			}, 30*time.Second)
			if err != nil {
				s.Fatal("Finding Time range PopUpButton node failed: ", err)
			}
			defer trNode.Release(ctx)

			if err := trNode.LeftClick(ctx); err != nil {
				s.Fatal("Failed to click address bar: ", err)
			}

			// Find the All time ListBoxOption and click it.
			atNode, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{
				Role: ui.RoleTypeListBoxOption,
				Name: "All time",
			}, 30*time.Second)
			if err != nil {
				s.Fatal("Finding All time ListBoxOption node failed: ", err)
			}
			defer atNode.Release(ctx)

			if err := atNode.LeftClick(ctx); err != nil {
				s.Fatal("Failed to click list bar: ", err)
			}

			// Find the clear data button node and click it.
			cbNode, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{
				Role: ui.RoleTypeButton,
				Name: "Clear data",
			}, 30*time.Second)
			if err != nil {
				s.Fatal("Finding clear data button node failed: ", err)
			}
			defer cbNode.Release(ctx)

			if err := cbNode.LeftClick(ctx); err != nil {
				s.Fatal("Failed to click clear bar: ", err)
			}

			// Wait for the dialog to close.
			if err := ui.WaitUntilGone(ctx, tconn, ui.FindParams{
				Role: ui.RoleTypeDialog,
				Name: "Clear browsing data",
			}, 30*time.Second); err != nil {
				s.Fatal("Clear browsing data dialog not closed: ", err)
			}

			// Open a website to create a browsing history entry.
			conn, err := cr.NewConn(ctx, server.URL+"/saving_browser_history_disabled.html")
			if err != nil {
				s.Fatal("Failed to connect to chrome: ", err)
			}
			defer conn.Close()

			hconn, err := cr.NewConn(ctx, "chrome://history")
			if err != nil {
				s.Fatal("Failed to connect to chrome: ", err)
			}
			defer hconn.Close()

			// Find a text inidcating there is no history.
			histFound := false
			bhNode, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{
				Role: ui.RoleTypeStaticText,
				Name: "Your browsing history appears here",
			}, 30*time.Second)
			if errors.Is(err, ui.ErrNodeDoesNotExist) {
				histFound = true
			} else if err != nil {
				s.Fatal("Finding text node failed: ", err)
			} else {
				defer bhNode.Release(ctx)
			}

			if histFound != param.enabled {
				s.Errorf("Unexpected existence of browser history found: got %t; want %t", histFound, param.enabled)
			}
		})
	}
}
