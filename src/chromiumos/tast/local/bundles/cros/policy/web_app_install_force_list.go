// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/policy/pre"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui/launcher"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: WebAppInstallForceList,
		Desc: "Behavior of WebAppInstallForceList policy",
		Contacts: []string{
			"alexanderhartl@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          pre.User,
		Data:         []string{"web_app_install_force_list_index.html"},
	})
}

func WebAppInstallForceList(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*pre.PreData).Chrome
	fdms := s.PreValue().(*pre.PreData).FakeDMS

	htmlfile := "web_app_install_force_list_index.html"

	// Get the path of the index.html for our PWA.
	path := s.DataPath(htmlfile)
	path = path[0 : len(path)-len(htmlfile)]

	// Start a http file server.
	_, addr, err := newPWAServer(ctx, path)
	if err != nil {
		s.Fatal("Failed to start server: ", err)
	}

	name := "Test PWA"
	value := &policy.WebAppInstallForceList{
		Val: []*policy.WebAppInstallForceListValue{
			{
				Url:                    fmt.Sprintf("http://%s/%s", addr, htmlfile),
				DefaultLaunchContainer: "window",
			},
		},
	}

	s.Run(ctx, name, func(ctx context.Context, s *testing.State) {
		// Update policies.
		if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{value}); err != nil {
			s.Fatal("Failed to update policies: ", err)
		}

		tconn, err := cr.TestAPIConn(ctx)
		if err != nil {
			s.Fatal("Cannot get TestConn: ", err)
		}

		// Poll until the PWA is successfully launched or the test ends.
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if err := launcher.SearchAndLaunch(ctx, tconn, name); err != nil {
				errors.Wrapf(err, "couldn't launch %s", name)
			}

			windows, err := ash.GetAllWindows(ctx, tconn)
			if err != nil {
				errors.Wrap(err, "failed to get windows")
			}

			for _, window := range windows {
				if window.Title == name {
					return nil
				}
			}
			return errors.New("couldn't find a window with the PWA")
		}, nil); err != nil {
			s.Fatal("PWA wasn't installed: ", err)
		}
	})
}

// newPWAServer creates a new http file server.
func newPWAServer(ctx context.Context, path string) (*http.Server, string, error) {
	srv := &http.Server{}

	srv.Handler = http.FileServer(http.Dir(path))

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, "", errors.Wrap(err, "failed to create listener for http server")
	}

	port := listener.Addr().(*net.TCPAddr).Port
	srv.Addr = fmt.Sprintf("127.0.0.1:%d", port)

	go func() {
		if err := srv.Serve(listener); err != http.ErrServerClosed {
			testing.ContextLog(ctx, "ExternalDataServer HTTP server failed: ", err)
		}
	}()

	return srv, srv.Addr, nil
}
