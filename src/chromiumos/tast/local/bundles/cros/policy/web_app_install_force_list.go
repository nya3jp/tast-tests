// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/bundles/cros/policy/pre"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui/launcher"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

const htmlFile = "web_app_install_force_list_index.html"
const manifestFile = "web_app_install_force_list_manifest.json"
const serviceWorkerFile = "web_app_install_force_list_service-worker.js"
const smallIconFile = "web_app_install_force_list_icon-192x192.png"
const bigIconFile = "web_app_install_force_list_icon-512x512.png"

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
		Data:         []string{htmlFile, manifestFile, serviceWorkerFile, smallIconFile, bigIconFile},
	})
}

func WebAppInstallForceList(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*pre.PreData).Chrome
	fdms := s.PreValue().(*pre.PreData).FakeDMS

	// Create a temporary directory and copy the PWA files to it
	td, err := ioutil.TempDir("", "")
	if err != nil {
		s.Fatal("Failed to create a temp dir: ", err)
	}
	defer os.RemoveAll(td)
	if err := fsutil.CopyFile(s.DataPath(htmlFile), filepath.Join(td, "index.html")); err != nil {
		s.Fatalf("Failed to copy %s: %v", htmlFile, err)
	}
	if err := fsutil.CopyFile(s.DataPath(manifestFile), filepath.Join(td, manifestFile)); err != nil {
		s.Fatalf("Failed to copy %s: %v", manifestFile, err)
	}
	if err := fsutil.CopyFile(s.DataPath(serviceWorkerFile), filepath.Join(td, serviceWorkerFile)); err != nil {
		s.Fatalf("Failed to copy %s: %v", serviceWorkerFile, err)
	}
	if err := fsutil.CopyFile(s.DataPath(smallIconFile), filepath.Join(td, smallIconFile)); err != nil {
		s.Fatalf("Failed to copy %s: %v", smallIconFile, err)
	}
	if err := fsutil.CopyFile(s.DataPath(bigIconFile), filepath.Join(td, bigIconFile)); err != nil {
		s.Fatalf("Failed to copy %s: %v", bigIconFile, err)
	}

	server := httptest.NewServer(http.FileServer(http.Dir(td)))
	defer server.Close()

	name := "Test PWA"
	value := &policy.WebAppInstallForceList{Val: []*policy.WebAppInstallForceListValue{{
		Url:                    server.URL,
		DefaultLaunchContainer: "window"}}}

	// Update policies.
	if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{value}); err != nil {
		s.Fatal("Failed to update policies: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Cannot get TestConn: ", err)
	}

	// Wait until the PWA is installed.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := launcher.SearchAndLaunch(ctx, tconn, name); err != nil {
			return testing.PollBreak(errors.Wrapf(err, "couldn't launch %s", name))
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
		return errors.New("couldn't find a window with the PWA")
	}, nil); err != nil {
		s.Error("PWA wasn't installed: ", err)
	}
}
