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
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/pre"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: PromptForDownloadLocation,
		Desc: "Test behavior of PromptForDownloadLocation policy: checking the a prompt for the download location appears based on the value of the policy",
		Contacts: []string{
			"alexanderhartl@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          pre.User,
		Data:         []string{"prompt_for_download_location.html", "prompt_for_download_location.zip"},
	})
}

// PromptForDownloadLocation tests the PromptForDownloadLocation policy.
func PromptForDownloadLocation(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*pre.PreData).Chrome
	fdms := s.PreValue().(*pre.PreData).FakeDMS

	// Connect to Test API to use it with the ui library.
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
		name           string
		wantAsk        bool
		wantRestricted bool                              // wantRestricted is the wanted restriction state of the toggle button in the download settings.
		wantChecked    ui.CheckedState                   // wantChecked is the wanted checked state of the toggle button in the download settings.
		value          *policy.PromptForDownloadLocation // value is the value of the policy.
	}{
		{
			name:           "unset",
			wantAsk:        false,
			wantRestricted: false,
			wantChecked:    ui.CheckedStateFalse,
			value:          &policy.PromptForDownloadLocation{Stat: policy.StatusUnset},
		},
		{
			name:           "disabled",
			wantAsk:        false,
			wantRestricted: true,
			wantChecked:    ui.CheckedStateFalse,
			value:          &policy.PromptForDownloadLocation{Val: false},
		},
		{
			name:           "enabled",
			wantAsk:        true,
			wantRestricted: true,
			wantChecked:    ui.CheckedStateTrue,
			value:          &policy.PromptForDownloadLocation{Val: true},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndRefresh(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Start a download.
			conn, err := cr.NewConn(ctx, server.URL+"/prompt_for_download_location.html")
			if err != nil {
				s.Fatal("Failed to start download: ", err)
			}
			defer conn.Close()

			err = conn.Exec(ctx, `document.getElementById('dlink').click()`)
			if err != nil {
				s.Fatal("Failed to execute JS expression: ", err)
			}

			// Check whether we get a prompt for the download location or if the file gets downloaded to the default folder.
			asked := false
			if err := testing.Poll(ctx, func(ctx context.Context) error {

				// Check if there is a prompt for the download location.
				params := ui.FindParams{
					Role: ui.RoleTypeWindow,
					Name: "Save file as",
				}
				if exist, err := ui.Exists(ctx, tconn, params); err != nil && !errors.Is(err, ui.ErrNodeDoesNotExist) {
					testing.PollBreak(errors.Wrap(err, "finding prompt for download location failed"))
				} else if exist {
					asked = true
					return nil
				}

				// Check if the file was downloaded.
				if _, err := os.Stat(filesapp.DownloadPath); err != nil && !os.IsNotExist(err) {
					testing.PollBreak(errors.Wrap(err, "finding downloaded file failed"))
				} else {
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

			// Open settings page where the affected toggle button can be found.
			sconn, err := cr.NewConn(ctx, "chrome://settings/downloads")
			if err != nil {
				s.Fatal("Failed to connect to the settings page: ", err)
			}
			defer sconn.Close()

			paramsTB := ui.FindParams{
				Role: ui.RoleTypeToggleButton,
				Name: "Ask where to save each file before downloading",
			}
			// Find the toggle button node.
			nodeTB, err := ui.FindWithTimeout(ctx, tconn, paramsTB, 15*time.Second)
			if err != nil {
				s.Fatal("Finding toggle button node failed: ", err)
			}
			defer nodeTB.Release(ctx)

			// Check the restriction setting of the toggle button.
			if restricted := (nodeTB.Restriction == ui.RestrictionDisabled || nodeTB.Restriction == ui.RestrictionReadOnly); restricted != param.wantRestricted {
				s.Errorf("Unexpected toggle button restriction in the settings: got %t; want %t", restricted, param.wantRestricted)
			}

			if nodeTB.Checked != param.wantChecked {
				s.Errorf("Unexpected toggle button checked state in the settings: got %s; want %s", nodeTB.Checked, param.wantChecked)
			}
		})
	}
}
