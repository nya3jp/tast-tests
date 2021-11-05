// Copyright 2021 The Chromium OS Authors. All rights reserved.
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

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

const fileBasename = "auto_open_file_types"

func init() {
	testing.AddTest(&testing.Test{
		Func: AutoOpenFileTypes,
		Desc: "Test behavior of AutoOpenFileTypes policy: checking if a file is automatically opened after downloading it, depending on the file extension and the value of the policy",
		Contacts: []string{
			"neis@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Fixture: fixture.ChromePolicyLoggedIn,
			Val:     lacros.ChromeTypeChromeOS,
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val:               lacros.ChromeTypeLacros,
		}},
		Data: []string{fileBasename + ".html", fileBasename + ".txt", fileBasename + ".unsupported"},
	})
}

func AutoOpenFileTypes(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	defer deleteDownloads(s)

	for _, param := range []struct {
		name     string
		autoOpen []string
		value    *policy.AutoOpenFileTypes
	}{
		{
			name:  "unset",
			value: &policy.AutoOpenFileTypes{Stat: policy.StatusUnset},
		},
		{
			name:  "set_txt",
			value: &policy.AutoOpenFileTypes{Val: []string{"txt"}},
		},
		{
			name:  "set_unsupported",
			value: &policy.AutoOpenFileTypes{Val: []string{"unsupported"}},
		},
		{
			name:  "set_txt+unsupported",
			value: &policy.AutoOpenFileTypes{Val: []string{"txt", "unsupported"}},
		},
	} {
		// We loop over the test cases here instead of inside because
		// we want to avoid the "Download multiple files" prompt.
		for _, extension := range []string{"txt", "unsupported"} {
			s.Run(ctx, param.name+"_"+extension, func(ctx context.Context, s *testing.State) {
				defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

				// Perform cleanup.
				if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
					s.Fatal("Failed to clean up: ", err)
				}

				// Update policies.
				if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
					s.Fatal("Failed to update policies: ", err)
				}

				// TODO(crbug.com/1254152): Modify browser setup after creating the new browser package.
				// Setup browser based on the chrome type.
				_, l, br, err := lacros.Setup(ctx, s.FixtValue(), s.Param().(lacros.ChromeType))
				if err != nil {
					s.Fatal("Failed to open the browser: ", err)
				}
				defer lacros.CloseLacrosChrome(cleanupCtx, l)

				// Open page.
				conn, err := br.NewConn(ctx, server.URL+"/"+fileBasename+".html")
				if err != nil {
					s.Fatal("Failed to open page: ", err)
				}
				defer conn.Close()

				// XXX Use browser package to encapsulate Lacros logic.
				if l != nil {
					testing.Sleep(ctx, 1*time.Second) // XXX Avoid this.
					if err := l.CloseAboutBlank(ctx, tconn, 0); err != nil {
						s.Fatal("Failed to close about:blank: ", err)
					}
				}

				// Start download.
				if err := conn.Eval(ctx, "document.getElementById('"+extension+"').click()", nil); err != nil {
					s.Fatal("Failed to execute JS expression: ", err)
				}

				// Ensure file was downloaded.
				if err := testing.Poll(ctx, func(_ context.Context) error {
					if _, err := os.Stat(filesapp.DownloadPath + fileBasename + "." + extension); err != nil {
						if os.IsNotExist(err) {
							return errors.New("file not (yet?) found")
						}
						return testing.PollBreak(errors.Wrap(err, "failed to stat file"))
					}
					// Remove downloaded file.
					if err := os.Remove(filesapp.DownloadPath + fileBasename + "." + extension); err != nil {
						return testing.PollBreak(errors.Wrap(err, "failed to remove file"))
					}
					return nil
				}, &testing.PollOptions{
					Timeout: 5 * time.Second,
				}); err != nil {
					s.Fatal("Failed to check if file was downloaded: ", err)
				}

				// Ensure downloaded file was openend automatically if appropriate.
				expectedTabCount := 1
				if shouldAutoOpen(extension, param.value) {
					expectedTabCount++
				}
				actualTabCount := getTabCount(ctx, tconn, s)
				// XXX Use browser package to encapsulate Lacros logic.
				if l != nil {
					tconn, err := l.TestAPIConn(ctx)
					if err != nil {
						s.Fatal("Failed to create Test API connection: ", err)
					}
					actualTabCount = getTabCount(ctx, tconn, s)
				}
				if actualTabCount != expectedTabCount {
					s.Log(":(")
					testing.Sleep(ctx, 20*time.Second)
					s.Fatalf("Unexpected number of tabs: got %d, expected %d", actualTabCount, expectedTabCount)
				}
			})
		}
	}
}

// - Lacros doesn't open files in a new Lacros tab but in Ash.
// - If asked to open unsupported files, Ash will show some error message after the download. Lacros doesn't.
// - In Ash, downloaded files do not pile up in a designated bar at the bottom of the browser window.
// - Is the notion of "supported" based on extension name or on mime type or on content?
// - An empty file is always opened!?

func deleteDownloads(s *testing.State) {
	files, err := ioutil.ReadDir(filesapp.DownloadPath)
	if err != nil {
		s.Fatal("Failed to read Downloads directory: ", err)
	}
	for _, file := range files {
		path := filepath.Join(filesapp.DownloadPath, file.Name())
		if err = os.RemoveAll(path); err != nil {
			s.Fatalf("Failed to remove file %v: %s", path, err)
		}
	}
}

func shouldAutoOpen(extension string, policy *policy.AutoOpenFileTypes) bool {
	if extension == "unsupported" {
		// Unsupported file types can't be auto-opened, no matter what the policy says.
		// XXX: Check that error is shown if policy says auto-open? Currently not true for Lacros.
		return false
	}
	for _, ext := range policy.Val {
		if ext == extension {
			return true
		}
	}
	return false
	// XXX: Look at policy.Stat?
}

func getTabCount(ctx context.Context, tconn *chrome.TestConn, s *testing.State) int {
	tabCount := 0
	err := tconn.Eval(ctx, `(async () => {
		const tabs = await tast.promisify(chrome.tabs.query)({});
		return tabs.length;
	})()`, &tabCount)
	if err != nil {
		s.Fatal("Failed to execute JS expression: ", err)
	}
	return tabCount
}
