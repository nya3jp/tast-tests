// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/policy/pre"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DefaultPopupsSetting,
		Desc: "Behavior of DefaultPopupsSetting policy",
		Contacts: []string{
			"alexanderhartl@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          pre.User,
		Data:         []string{"default_popups_setting_index.html", "default_popups_setting_popup.html"},
	})
}

func DefaultPopupsSetting(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*pre.PreData).Chrome
	fdms := s.PreValue().(*pre.PreData).FakeDMS

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	policies := []policy.Policy{
		&policy.DefaultPopupsSetting{
			Val: 1,
		},
	}

	// Update policies.
	if err := policyutil.ServeAndVerify(ctx, fdms, cr, policies); err != nil {
		s.Fatal("Failed to update policies: ", err)
	}

	conn, err := cr.NewConn(ctx, server.URL+"/default_popups_setting_index.html")
	if err != nil {
		s.Fatal("Failed to connect to chrome: ", err)
	}
	defer conn.Close()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get TestConn: ", err)
	}

	// Wait until the popup window is opened
	if err := testing.Poll(ctx, func(ctx context.Context) error {

		windows, err := ash.GetAllWindows(ctx, tconn)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get windows"))
		}

		for _, window := range windows {
			if strings.Contains(window.Title, "PopUpWindow") {
				return nil
			}
		}

		return errors.New("failed to find the popup window")
	}, nil); err != nil {
		s.Error("Popup window not opened: ", err)
	}
}
