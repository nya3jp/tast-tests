// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/policy/pre"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui/launcher"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/testexec"
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
		Data:         []string{"web_app_install_force_list_index.html", "web_app_install_force_list_manifest.json", "web_app_install_force_list_icon-192x192.png", "web_app_install_force_list_icon-512x512.png", "web_app_install_force_list_service-worker.js"},
	})
}

func WebAppInstallForceList(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*pre.PreData).Chrome
	fdms := s.PreValue().(*pre.PreData).FakeDMS

	// get path of the index.html for our PWA
	path := s.DataPath("web_app_install_force_list_index.html")
	path = path[0 : len(path)-37]

	// copy web_app_install_force_list_index.html to index.html for our http server
	ih := fmt.Sprintf(path, "index.html")
	if err := testexec.CommandContext(ctx, "cp", s.DataPath("web_app_install_force_list_index.html"), ih).Run(
		testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to copy ", s.DataPath("web_app_install_force_list_index.html"), " to ", ih)
	}

	// start a http server
	_, port, err := newPWAServer(ctx, path)
	if err != nil {
		s.Fatal("Failed to start server: ", err)
	}

	// the policy
	name := "Test PWA"
	value := &policy.WebAppInstallForceList{
		Val: []*policy.WebAppInstallForceListValue{
			{
				Url:                    fmt.Sprintf("http://localhost:%d", port),
				DefaultLaunchContainer: "window",
			},
		},
	}

	s.Run(ctx, name, func(ctx context.Context, s *testing.State) {
		//Update policies.
		if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{value}); err != nil {
			s.Fatal("Failed to update policies: ", err)
		}

		tconn, err := cr.TestAPIConn(ctx)
		if err != nil {
			s.Fatal("Cannot get TestConn: ", err)
		}

		// loop until the PWA is successfully launched or the test ends
		for {
			err = launcher.SearchAndLaunch(ctx, tconn, name)
			if err != nil {
				s.Fatal("Couldn't launch ", name, ": ", err)
			}

			windows, err := ash.GetAllWindows(ctx, tconn)
			if err != nil {
				s.Fatal("Failed to get windows: ", err)
			}

			for _, window := range windows {
				if window.Title == name {
					return
				}
				s.Log(window.Title, "does not equal ", name)
			}

			testing.Sleep(ctx, time.Second)
		}
	})
}

// newPWAServer creates a new http server.
func newPWAServer(ctx context.Context, path string) (*http.Server, int, error) {
	srv := &http.Server{}

	srv.Handler = http.FileServer(http.Dir(path))

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to create listener for http server")
	}

	port := listener.Addr().(*net.TCPAddr).Port
	srv.Addr = fmt.Sprintf("127.0.0.1:%d", port)

	go func() {
		if err := srv.Serve(listener); err != http.ErrServerClosed {
			testing.ContextLog(ctx, "ExternalDataServer HTTP server failed: ", err)
		}
	}()

	return srv, port, nil
}
