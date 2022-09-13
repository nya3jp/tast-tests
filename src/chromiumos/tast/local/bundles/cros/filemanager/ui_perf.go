// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package filemanager

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/cpu"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

type perfTestFunc = func(ctx context.Context, s *testing.State, files *filesapp.FilesApp, ew *input.KeyboardEventWriter, testCase, downloadsPath string, totalFiles int) error

type perfTest struct {
	// Each test can run with multiple sizes, use when the initial setup can be shared for all sizes.
	sizes []int
	// The function that simulate the user operation.
	testFunc perfTestFunc
	// The UMA name that should be collected after running `testFunc`.
	umaName string
	// Whether the test requires to install PWAs.
	installPwas bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         UIPerf,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "UI performance metrics for Files app",
		Contacts: []string{
			"benreich@chromium.org",
			"lucmult@chromium.org",
			"chromeos-files-syd@google.com",
		},
		Timeout:      4 * time.Minute,
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		Data:         []string{"pwa_manifest.json", "pwa_service.js", "pwa_index.html", "pwa_icon.png"},
		Params: []testing.Param{
			{
				Name: "directory_list",
				Val: perfTest{
					sizes:       []int{100, 1000},
					testFunc:    testDirectoryListing,
					umaName:     "FileBrowser.DirectoryListLoad.my_files.%d",
					installPwas: false,
				},
			},
			{
				Name: "list_apps",
				Val: perfTest{
					sizes:       []int{10},
					testFunc:    testAvailableAppsListing,
					umaName:     "FileBrowser.UpdateAvailableApps.%d",
					installPwas: true,
				},
				// Fixture:         "install100AppsWithFileHandlers",
			},
		},
	})
}

// installPWAForURL navigates to a PWA, attempts to install and returns the installed app ID.
func installPWAForURL(ctx context.Context, cr *chrome.Chrome, pwaURL string, timeout time.Duration) (string, error) {
	conn, err := cr.NewConn(ctx, pwaURL)
	if err != nil {
		return "", errors.Wrapf(err, "failed to open URL %q", pwaURL)
	}
	defer conn.Close()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to connect to test API")
	}

	// The installability checks occur asynchronously for PWAs.
	// Wait for the Install button to appear in the Chrome omnibox before installing.
	ui := uiauto.New(tconn)
	install := nodewith.ClassName("PwaInstallView").Role(role.Button)
	if err := ui.WithTimeout(timeout).WaitUntilExists(install)(ctx); err != nil {
		return "", errors.Wrap(err, "failed to wait for the install button in the omnibox")
	}

	evalString := fmt.Sprintf("tast.promisify(chrome.autotestPrivate.installPWAForCurrentURL)(%d)", timeout.Milliseconds())

	var appID string
	if err := tconn.Eval(ctx, evalString, &appID); err != nil {
		return "", errors.Wrap(err, "failed to run installPWAForCurrentURL")
	}

	return appID, nil
}

func UIPerf(ctx context.Context, s *testing.State) {
	params := s.Param().(perfTest)

	//options := s.FixtValue().([]chrome.Option)
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Start Chrome with SWA flag.
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Cannot start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	downloadsPath, err := cryptohome.DownloadsPath(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to retrieve users Downloads path: ", err)
	}

	// Get Test API connection.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	const (
		localServerIP   = "127.0.0.1"
		localServerPort = 8080
		installTimeout  = 15 * time.Second
		pwaWindowTitle  = "PWA Open TXT Test App - Test PWA"
	)

	if params.installPwas {
		mux := http.NewServeMux()
		fs := http.FileServer(s.DataFileSystem())
		mux.Handle("/", fs)

		server := &http.Server{Addr: fmt.Sprintf(":%v", localServerPort), Handler: mux}
		go func() {
			if err := server.ListenAndServe(); err != http.ErrServerClosed {
				s.Fatal("Failed to create local server: ", err)
			}
		}()
		defer server.Shutdown(ctx)

		// Install 10 PWAs
		for appIdx := 1; appIdx <= 10; appIdx++ {
			_, err := installPWAForURL(ctx, cr, fmt.Sprintf("http://127.0.0.%d:%v/pwa_index.html", appIdx, localServerPort), installTimeout)
			if err != nil {
				s.Fatalf("Failed to install PWA %d, %s:", appIdx, err)
			}
			window, err := ash.WaitForAnyWindowWithTitle(ctx, tconn, pwaWindowTitle)
			if err != nil {
				s.Fatalf("Failed to wait for PWA window with title %s: %s", pwaWindowTitle, err)
			}
			window.CloseWindow(ctx, tconn)
		}
	}

	// Launch the Files app.
	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Launching the Files App failed: ", err)
	}

	// Get a keyboard handle to use accelerators such as Ctrl+A.
	ew, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Error creating keyboard: ", err)
	}
	defer ew.Close()

	pv := perf.NewValues()

	// Measure how long to display 10 and 100 available apps for a selected file.
	for _, size := range params.sizes {
		testCase := fmt.Sprintf("%d_size", size)
		s.Logf("Running test: %s", testCase)
		uma := fmt.Sprintf(params.umaName, size)
		s.Run(ctx, testCase, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, testCase)

			// Run the directory listing test and observe the histogram for output.
			histograms, err := metrics.Run(ctx, tconn, func(ctx context.Context) error {
				//return testAvailableAppsListing(ctx, s, files, ew, testCase, downloadsPath, totalApps)
				return params.testFunc(ctx, s, files, ew, testCase, downloadsPath, size)
			}, uma)
			if err != nil {
				s.Fatal("Failed to retrieve histogram data: ", err)
			}

			if len(histograms) != 1 {
				s.Fatalf("Failed to record histogram, got %d want 1", len(histograms))
			}
			if histograms[0].Sum == 0 {
				s.Fatal("Failed to record a histogram value for: ", uma)
			}
		})
	}

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	} else {
		s.Log("Saved perf data")
	}
}

func createDirectoryListing(ctx context.Context, folderPath string, files int) error {
	for i := 0; i < files; i++ {
		filePath := filepath.Join(folderPath, fmt.Sprintf("File-%d.txt", i))
		if err := ioutil.WriteFile(filePath, []byte("blah"), 0644); err != nil {
			return errors.Wrapf(err, "failed to create file with path %q", filePath)
		}
	}
	return nil
}

func testDirectoryListing(ctx context.Context, s *testing.State, files *filesapp.FilesApp, ew *input.KeyboardEventWriter, testCase, downloadsPath string, totalFiles int) error {
	folderPath := filepath.Join(downloadsPath, testCase)
	if err := os.MkdirAll(folderPath, 0644); err != nil {
		return errors.Wrapf(err, "failed to create directory %q", folderPath)
	}
	defer os.RemoveAll(folderPath)
	if err := createDirectoryListing(ctx, folderPath, totalFiles); err != nil {
		return errors.Wrapf(err, "failed to create folder with name %q", folderPath)
	}

	// Wait until cpu idle before starting performance measures.
	if err := cpu.WaitUntilIdle(ctx); err != nil {
		return errors.Wrap(err, "failed to wait for CPU to be idle")
	}

	filesAllLoaded := func(ctx context.Context) error {
		return testing.Poll(ctx, func(ctx context.Context) error {
			if err := ew.Accel(ctx, "ctrl+A"); err != nil {
				return testing.PollBreak(errors.Wrap(err, "failed to select all files using Ctrl+A"))
			}
			return files.Exists(nodewith.Name(fmt.Sprintf("%d files selected", totalFiles)).Role(role.StaticText))(ctx)
			// The Interval is 2s as pressing Ctrl+A has a UI performance hit, it will
			// slow down the listing whilst the selection is tallied, want to avoid
			// calling this too often to ensure the data is less tainted.
		}, &testing.PollOptions{Timeout: time.Minute, Interval: 2 * time.Second})
	}

	return uiauto.Combine("Navigate to folder and keep selecting all until total count matches expected",
		files.OpenPath(filesapp.FilesTitlePrefix+filesapp.Downloads, filesapp.Downloads, testCase),
		filesAllLoaded,
	)(ctx)
}

func testAvailableAppsListing(ctx context.Context, s *testing.State, files *filesapp.FilesApp, ew *input.KeyboardEventWriter, testCase, downloadsPath string, totalApps int) error {
	totalFiles := 100
	folderPath := filepath.Join(downloadsPath, testCase)
	if err := os.MkdirAll(folderPath, 0644); err != nil {
		return errors.Wrapf(err, "failed to create directory %q", folderPath)
	}
	defer os.RemoveAll(folderPath)

	// Setup 100 text files to be selected.
	if err := createDirectoryListing(ctx, folderPath, totalFiles); err != nil {
		return errors.Wrapf(err, "failed to create folder with name %q", folderPath)
	}

	// Wait until cpu idle before starting performance measures.
	if err := cpu.WaitUntilIdle(ctx); err != nil {
		return errors.Wrap(err, "failed to wait for CPU to be idle")
	}

	selectFile := func(ctx context.Context) error {
		return testing.Poll(ctx, func(ctx context.Context) error {
			if err := ew.Accel(ctx, "ctrl+A"); err != nil {
				return testing.PollBreak(errors.Wrap(err, "failed to select all files using Ctrl+A"))
			}
			return files.Exists(nodewith.Name(fmt.Sprintf("%d files selected", totalFiles)).Role(role.StaticText))(ctx)
			// The Interval is 2s as pressing Ctrl+A has a UI performance hit, it will
			// slow down the listing whilst the selection is tallied, want to avoid
			// calling this too often to ensure the data is less tainted.
		}, &testing.PollOptions{Timeout: time.Minute, Interval: 2 * time.Second})
	}

	checkNumApps := func(ctx context.Context) error {
		return testing.Poll(ctx, func(ctx context.Context) error {
			nodes, err := files.NodesInfo(ctx, nodewith.Role(role.MenuItem).Ancestor(nodewith.Role(role.Menu).Name(filesapp.OpenWith)))
			if err != nil {
				return testing.PollBreak(errors.Wrap(err, "failed to check the menuitems for Open with menu"))
			}
			// The menu might show more apps, like built-in apps in addition to the PWAs installed in the test.
			if len(nodes) >= totalApps {
				return nil
			}
			return errors.Errorf("waiting for %d apps, got %d ", totalApps, len(nodes))
		}, &testing.PollOptions{Timeout: time.Minute, Interval: 2 * time.Second})
	}

	return uiauto.Combine("Navigate to folder, select all files and wait until the total apps count matches expected",
		files.OpenPath(filesapp.FilesTitlePrefix+filesapp.Downloads, filesapp.Downloads, testCase),
		selectFile,
		files.ExpandOpenDropdown(),
		checkNumApps,
		// Use OpenPath() to dismiss the dropdown menu, otherwise it interferes with next test.
		files.OpenPath(filesapp.FilesTitlePrefix+filesapp.Downloads, filesapp.Downloads, testCase),
	)(ctx)
}
