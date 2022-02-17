// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         FileSystemWriteBlockedForUrls,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checking if file system writes are blocked depending on the value of this policy",
		Contacts: []string{
			"cmfcmf@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Fixture: fixture.ChromePolicyLoggedIn,
			Val:     browser.TypeAsh,
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val:               browser.TypeLacros,
		}},
		Data: []string{"file_system_write_blocked_for_urls_index.html"},
	})
}

// FileSystemWriteBlockedForUrls tests the FileSystemWriteBlockedForUrls policy.
func FileSystemWriteBlockedForUrls(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Reserve 10 seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	matchingServer := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer matchingServer.Close()
	nonMatchingServer := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer nonMatchingServer.Close()

	const indexFileName = "file_system_write_blocked_for_urls_index.html"
	matchingURL := fmt.Sprintf("%s/%s", matchingServer.URL, indexFileName)
	nonMatchingURL := fmt.Sprintf("%s/%s", nonMatchingServer.URL, indexFileName)

	for _, param := range []struct {
		name                string
		url                 string
		wantFileSystemWrite bool
		policy              *policy.FileSystemWriteBlockedForUrls
	}{
		{
			name:                "blocked_matching",
			url:                 matchingURL,
			wantFileSystemWrite: false,
			policy:              &policy.FileSystemWriteBlockedForUrls{Val: []string{matchingURL}},
		},
		{
			name:                "blocked_non_matching",
			url:                 nonMatchingURL,
			wantFileSystemWrite: true,
			policy:              &policy.FileSystemWriteBlockedForUrls{Val: []string{matchingURL}},
		},
		{
			name:                "unset",
			url:                 matchingURL,
			wantFileSystemWrite: true,
			policy:              &policy.FileSystemWriteBlockedForUrls{Stat: policy.StatusUnset},
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

			// Cleanup the file _after_ the browser has been closed, to be sure that
			// the browser is not still in the process of writing the file.
			filePath := path.Join(filesapp.MyFilesPath, "test-file")
			if param.wantFileSystemWrite {
				defer os.Remove(filePath)
			}

			// Setup browser based on the chrome type.
			br, closeBrowser, err := browserfixt.SetUp(ctx, s.FixtValue(), s.Param().(browser.Type))
			if err != nil {
				s.Fatal("Failed to open the browser: ", err)
			}
			defer closeBrowser(cleanupCtx)
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			conn, err := br.NewConn(ctx, param.url)
			if err != nil {
				s.Fatal("Failed to open website: ", err)
			}
			defer conn.Close()

			// Connect to Test API to use it with the UI library.
			tconn, err := cr.TestAPIConn(ctx)
			if err != nil {
				s.Fatal("Failed to create Test API connection: ", err)
			}

			// Attempt to open the file picker by clicking the HTML link that triggers
			// window.showSaveFilePicker(). We cannot use conn.Eval() for this,
			// because opening the file picker must be triggered by a user gesture for
			// security reasons.
			ui := uiauto.New(tconn)
			if err := ui.LeftClick(nodewith.Role(role.Link).Name("showSaveFilePicker"))(ctx); err != nil {
				s.Error("Failed to click link to open the save file picker: ", err)
			}

			// Save the file using the file picker dialog if a file picker dialog was
			// supposed to open, otherwise verify that no dialog has opened.
			saveFilePickerNode := nodewith.Role(role.Dialog).Name("Save file as").ClassName("RootView")
			if param.wantFileSystemWrite {
				if err := uiauto.Combine(
					"save file to disk",
					ui.WaitUntilExists(saveFilePickerNode),
					ui.LeftClick(nodewith.Role(role.Button).Name("Save")),
				)(ctx); err != nil {
					s.Error("Unable to save file using save file picker: ", err)
				}

				// Wait for the file to be written to disk and check its contents.
				if err := testing.Poll(ctx, func(ctx context.Context) error {
					fileContent, err := ioutil.ReadFile(filePath)
					if err != nil {
						return err
					}
					if string(fileContent) != "test" {
						return errors.Errorf("File contains invalid content, expected 'test', got: %s", string(fileContent))
					}
					return nil
				}, &testing.PollOptions{Timeout: 20 * time.Second}); err != nil {
					s.Error("Failed to check file on disk: ", err)
				}
			} else {
				if err := ui.EnsureGoneFor(saveFilePickerNode, 5*time.Second)(ctx); err != nil {
					s.Error("Save file picker opened unexpectedly: ", err)
				}
			}

			// Check if the file picker was shown and successfully closed by
			// inspecting what value the filePickerShownPromise promise resolved to.
			var filePickerShown bool
			if err := conn.Eval(ctx, "window.filePickerShownPromise", &filePickerShown); err != nil {
				s.Fatal("Failed to evaluate window.filePickerShownPromise: ", err)
			}

			if filePickerShown != param.wantFileSystemWrite {
				s.Errorf("Unexpected showSaveFilePicker behavior: got %t; want %t", filePickerShown, param.wantFileSystemWrite)
			}
		})
	}
}
