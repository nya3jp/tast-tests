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
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/local/chrome/uiauto/faillog"
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
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
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
			s.Fatal("Failed to get files from Downloads directory")
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
		wantRestriction ui.RestrictionState               // wantRestriction is the wanted restriction state of the toggle button in the download settings.
		wantChecked     ui.CheckedState                   // wantChecked is the wanted checked state of the toggle button in the download settings.
		value           *policy.PromptForDownloadLocation // value is the value of the policy.
	}{
		{
			name:            "unset",
			wantAsk:         false,
			wantRestriction: ui.RestrictionNone,
			wantChecked:     ui.CheckedStateFalse,
			value:           &policy.PromptForDownloadLocation{Stat: policy.StatusUnset},
		},
		{
			name:            "disabled",
			wantAsk:         false,
			wantRestriction: ui.RestrictionDisabled,
			wantChecked:     ui.CheckedStateFalse,
			value:           &policy.PromptForDownloadLocation{Val: false},
		},
		{
			name:            "enabled",
			wantAsk:         true,
			wantRestriction: ui.RestrictionDisabled,
			wantChecked:     ui.CheckedStateTrue,
			value:           &policy.PromptForDownloadLocation{Val: true},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeOnErrorToFile(ctx, s.OutDir(), s.HasError, tconn, "ui_tree_"+param.name+".txt")

			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Open settings page where the affected toggle button can be found.
			conn, err := cr.NewConn(ctx, "chrome://settings/downloads")
			if err != nil {
				s.Fatal("Failed to connect to the settings page: ", err)
			}
			defer conn.Close()

			if err := policyutil.VerifySettingsNode(ctx, tconn,
				ui.FindParams{
					Role: ui.RoleTypeToggleButton,
					Name: "Ask where to save each file before downloading",
				},
				ui.FindParams{
					Attributes: map[string]interface{}{
						"restriction": param.wantRestriction,
						"checked":     param.wantChecked,
					},
				},
			); err != nil {
				s.Error("Unexpected settings state: ", err)
			}

			// Start a download.
			if err := conn.Navigate(ctx, server.URL+"/prompt_for_download_location.html"); err != nil {
				s.Fatal("Failed to start download: ", err)
			}

			if err := conn.Eval(ctx, `document.getElementById('dlink').click()`, nil); err != nil {
				s.Fatal("Failed to execute JS expression: ", err)
			}

			// Check whether we get a prompt for the download location or if the file gets downloaded to the default folder.
			params := ui.FindParams{
				Role: ui.RoleTypeWindow,
				Name: "Save file as",
			}
			asked := false
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				// Check if there is a prompt for the download location.
				if exist, err := ui.Exists(ctx, tconn, params); err != nil && !errors.Is(err, ui.ErrNodeDoesNotExist) {
					return testing.PollBreak(errors.Wrap(err, "finding prompt for download location failed"))
				} else if exist {
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
