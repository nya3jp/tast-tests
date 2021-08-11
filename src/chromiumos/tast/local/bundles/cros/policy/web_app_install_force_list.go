// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"net/http"
	"net/http/httptest"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: WebAppInstallForceList,
		Desc: "Behavior of WebAppInstallForceList policy",
		Contacts: []string{
			"alexanderhartl@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "chromePolicyLoggedIn",
		Data:         []string{"web_app_install_force_list_index.html", "web_app_install_force_list_manifest.json", "web_app_install_force_list_service-worker.js", "web_app_install_force_list_icon-192x192.png", "web_app_install_force_list_icon-512x512.png"},
	})
}

func WebAppInstallForceList(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fdms := s.FixtValue().(*fixtures.FixtData).FakeDMS

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	policies := []policy.Policy{
		&policy.WebAppInstallForceList{
			Val: []*policy.WebAppInstallForceListValue{
				{
					Url:                    server.URL + "/web_app_install_force_list_index.html",
					DefaultLaunchContainer: "window",
				},
			},
		},
	}

	// Update policies.
	if err := policyutil.ServeAndVerify(ctx, fdms, cr, policies); err != nil {
		s.Fatal("Failed to update policies: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get TestConn: ", err)
	}
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	// Wait until the PWA is installed.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		const name = "Test PWA"
		if err := launcher.SearchAndLaunch(tconn, kb, name)(ctx); err != nil {
			return errors.Wrapf(err, "failed to launch %s", name)
		}

		windows, err := ash.GetAllWindows(ctx, tconn)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get windows"))
		}

		for _, window := range windows {
			if window.Title == name {
				return nil
			}
		}
		return errors.New("failed to find a window with the PWA")
	}, nil); err != nil {
		s.Error("PWA wasn't installed: ", err)
	}
}
