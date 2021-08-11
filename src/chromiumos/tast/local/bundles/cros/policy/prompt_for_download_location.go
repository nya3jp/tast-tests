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
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/checked"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/restriction"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: PromptForDownloadLocation,
		Desc: "Test behavior of PromptForDownloadLocation policy: checking if a prompt for the download location appears based on the value of the policy",
		Contacts: []string{
			"alexanderhartl@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "chromePolicyLoggedIn",
		Data:         []string{"prompt_for_download_location.html", "prompt_for_download_location.zip"},
	})
}

// PromptForDownloadLocation tests the PromptForDownloadLocation policy.
func PromptForDownloadLocation(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fdms := s.FixtValue().(*fixtures.FixtData).FakeDMS

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	defer func() { // Clean up Downloads directory.
		files, err := ioutil.ReadDir(filesapp.DownloadPath)
		if err != nil {
			s.Fatal("Failed to get files from Downloads directory: ", err)
		}
		for _, file := range files {
			if err = os.RemoveAll(filepath.Join(filesapp.DownloadPath, file.Name())); err != nil {
				s.Fatal("Failed to remove file: ", file.Name())
			}
		}
	}()

	for _, param := range []struct {
		name            string
		wantAsk         bool
		wantChecked     checked.Checked
		wantRestriction restriction.Restriction
		value           *policy.PromptForDownloadLocation
	}{
		{
			name:            "unset",
			wantAsk:         false,
			wantChecked:     checked.False,
			wantRestriction: restriction.None,
			value:           &policy.PromptForDownloadLocation{Stat: policy.StatusUnset},
		},
		{
			name:            "disabled",
			wantAsk:         false,
			wantChecked:     checked.False,
			wantRestriction: restriction.Disabled,
			value:           &policy.PromptForDownloadLocation{Val: false},
		},
		{
			name:            "enabled",
			wantAsk:         true,
			wantChecked:     checked.True,
			wantRestriction: restriction.Disabled,
			value:           &policy.PromptForDownloadLocation{Val: true},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			if err := policyutil.SettingsPage(ctx, cr, "downloads").
				SelectNode(ctx, nodewith.
					Name("Ask where to save each file before downloading").
					Role(role.ToggleButton)).
				Restriction(param.wantRestriction).
				Checked(param.wantChecked).
				Verify(); err != nil {
				s.Error("Unexpected settings state: ", err)
			}

			// Start a download.
			conn, err := cr.NewConn(ctx, server.URL+"/prompt_for_download_location.html")
			if err != nil {
				s.Fatal("Failed to start download: ", err)
			}
			defer conn.Close()

			if err := conn.Eval(ctx, `document.getElementById('dlink').click()`, nil); err != nil {
				s.Fatal("Failed to execute JS expression: ", err)
			}

			// Check whether we get a prompt for the download location or if the file gets downloaded to the default folder.
			ui := uiauto.New(tconn)
			downloadPrompt := nodewith.Name("Save file as").Role(role.Window)
			asked := false
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				// Check if there is a prompt for the download location.
				if err = ui.Exists(downloadPrompt)(ctx); err == nil {
					asked = true
					return nil
				}

				// Check if the file was downloaded.
				if _, err := os.Stat(filesapp.DownloadPath + "prompt_for_download_location.zip"); err != nil && !os.IsNotExist(err) {
					return testing.PollBreak(errors.Wrap(err, "finding downloaded file failed"))
				} else if !os.IsNotExist(err) {
					// Remove downloaded file.
					if err := os.Remove(filesapp.DownloadPath + "prompt_for_download_location.zip"); err != nil {
						return testing.PollBreak(errors.Wrap(err, "failed to remove prompt_for_download_location.zip"))
					}
					asked = false
					return nil
				}

				return errors.New("found no prompt and no file")

			}, &testing.PollOptions{
				Timeout: 30 * time.Second,
			}); err != nil {
				s.Fatal("Failed to check if asked for download location: ", err)
			}

			if asked != param.wantAsk {
				s.Errorf("Unexpected existence of download location prompt: got %t; want %t", asked, param.wantAsk)
			}
		})
	}
}
