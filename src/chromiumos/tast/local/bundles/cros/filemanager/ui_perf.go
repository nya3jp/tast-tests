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
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

type perfTestFunc = func(ctx context.Context, kb *input.KeyboardEventWriter, s *testing.State, files *filesapp.FilesApp, size int)

//func perfTestFunc(ctx context.Context, s *testing.State, files *filesapp.FilesApp, ew *input.KeyboardEventWriter, testCase, downloadsPath string, totalFiles int) error
type perfTest struct {
	size  int
	//testFunc perfTestFunc
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
		Params: 	  []testing.Param{{
			Name: "10_apps",
			Val:  perfTest{size: 10},
			// Fixture:         "install100AppsWithFileHandlers",

		}},
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
	//cr, err := chrome.New(ctx, options...)
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
		localServerIP        = "127.0.0.1"
		localServerPort      = 8080
		installTimeout       = 15 * time.Second
		//testAppID            = "cpdpbfelifklonephgpieimdpcecgoen"
		testAppID            = "bdilnhihdapfbpljlfhofahbnibbcaen"
	)

	mux := http.NewServeMux()
	fs := http.FileServer(s.DataFileSystem())
	mux.Handle("/", fs)

	server := &http.Server{Addr: fmt.Sprintf(":%v", localServerPort), Handler: mux}
	go func() {
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			s.Fatal("Failed to create local server: ", err)
		}
	}()

	// TODO: Re-enable it:
	// defer server.Shutdown(ctx)


	appID, err := installPWAForURL(ctx,cr,fmt.Sprintf("http://127.0.0.2:%v/pwa_index.html", localServerPort), installTimeout)
	if err != nil {
		s.Fatal("Failed to install PWA#2: ", err)
	}
	s.Logf("APPID: %s", appID)
	// if err := apps.InstallPWAForURL(ctx, tconn, cr.Browser(), fmt.Sprintf("http://127.0.0.2:%v/pwa_index.html", localServerPort), installTimeout); err != nil {
	// 	s.Fatal("Failed to install PWA#2: ", err)
	// }

	s.Log("INSTALLED PWA#2")
	appID, err = installPWAForURL(ctx,cr,fmt.Sprintf("http://127.0.0.1:%v/pwa_index.html", localServerPort), installTimeout)
	if err != nil {
	//if err := apps.InstallPWAForURL(ctx, tconn, cr.Browser(), fmt.Sprintf("http://%s:%v/pwa_index.html", localServerIP, localServerPort), installTimeout); err != nil {
		s.Fatal("Failed to install PWA: ", err)
	}
	s.Logf("APPID: %s", appID)

	s.Log("INSTALLED PWA")

	if err := ash.WaitForChromeAppInstalled(ctx, tconn, testAppID, 15*time.Second); err != nil {
		s.Fatal("Failed to wait for PWA to be installed: ", err)
	}

	if err := ash.WaitForApp(ctx, tconn, testAppID, 15*time.Second); err != nil {
		s.Fatal("Failed to wait for PWA to open: ", err)
	}

	s.Log("INSTALLED PWA #2")

	// Launch the Files app.
	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Launching the Files App failed: ", err)
	}

	s.Log("Files app LAUNCHED")

	// Get a keyboard handle to use accelerators such as Ctrl+A.
	ew, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Error creating keyboard: ", err)
	}
	defer ew.Close()

	pv := perf.NewValues()

	// // Measure how long 100 and 1,000 files take to render in Files app listbox.
	// for _, totalFiles := range []int{100, 1000} {
	// 	testCase := fmt.Sprintf("%d_files", totalFiles)
	// 	uma := fmt.Sprintf("FileBrowser.DirectoryListLoad.my_files.%d", totalFiles)
	// 	s.Run(ctx, testCase, func(ctx context.Context, s *testing.State) {
	// 		defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, testCase)

	// 		// Run the directory listing test and observe the histogram for output.
	// 		histograms, err := metrics.Run(ctx, tconn, func(ctx context.Context) error {
	// 			return testDirectoryListing(ctx, s, files, ew, testCase, downloadsPath, totalFiles)
	// 		}, uma)
	// 		if err != nil {
	// 			s.Fatal("Failed to retrieve histogram data: ", err)
	// 		}

	// 		if len(histograms) != 1 {
	// 			s.Fatalf("Failed to record histogram, got %d want 1", len(histograms))
	// 		}
	// 		if histograms[0].Sum == 0 {
	// 			s.Fatal("Failed to record a histogram value for: ", uma)
	// 		}

	// 		pv.Set(perf.Metric{
	// 			Name:      testCase,
	// 			Unit:      "ms",
	// 			Direction: perf.SmallerIsBetter,
	// 		}, float64(histograms[0].Sum))
	// 	})
	// }

	// Measure how long to display 10 and 100 available apps for a selected file.
	//for _, totalApps := range []int{10, 100} {
		totalApps := params.size;
		testCase := fmt.Sprintf("%d_apps", totalApps)
		uma := fmt.Sprintf("FileBrowser.UpdateAvailableApps.%d", totalApps)
		s.Run(ctx, testCase, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, testCase)

			// Run the directory listing test and observe the histogram for output.
			histograms, err := metrics.Run(ctx, tconn, func(ctx context.Context) error {
				return testAvailableAppsListing(ctx, s, files, ew, testCase, downloadsPath, totalApps)
			}, uma)
			if err != nil {
				s.Fatal("Failed to retrieve histogram data: ", err)
			}

			s.Log("Failed: %v", histograms)
			if len(histograms) != 1 {
				s.Fatalf("Failed to record histogram, got %d want 1", len(histograms))
			}
			if histograms[0].Sum == 0 {
				s.Fatal("Failed to record a histogram value for: ", uma)
			}
		})

	//}

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	} else {
		s.Log("Saved perf data")
	}
}

func createDirectoryListing(ctx context.Context, folderPath string, files int) error {
	for i := 0; i < files; i++ {
		filePath := filepath.Join(folderPath, fmt.Sprintf("File-%d.file", i))
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

	// TODO: Reenable it.
	// Wait until cpu idle before starting performance measures.
	//if err := cpu.WaitUntilIdle(ctx); err != nil {
	//	return errors.Wrap(err, "failed to wait for CPU to be idle")
	//}

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

	folderPath := filepath.Join(downloadsPath, testCase)
	if err := os.MkdirAll(folderPath, 0644); err != nil {
		return errors.Wrapf(err, "failed to create directory %q", folderPath)
	}
	// TODO: Reenable it.
	// defer os.RemoveAll(folderPath)

	// Setup one text file to be selected.
	fileName := "file.txt"
	filePath := filepath.Join(folderPath, fileName)
	if err := ioutil.WriteFile(filePath, []byte("blah"), 0644); err != nil {
		return errors.Wrapf(err, "failed to create file with path %q", filePath)
	}

	// TODO: Reenable it.
	// Wait until cpu idle before starting performance measures.
	//if err := cpu.WaitUntilIdle(ctx); err != nil {
	//	return errors.Wrap(err, "failed to wait for CPU to be idle")
	//}

	selectFile := func(ctx context.Context) error {
		return testing.Poll(ctx, func(ctx context.Context) error {
			return files.SelectFile(fileName)(ctx)
			//.Exists(nodewith.Name(fmt.Sprintf("%d files selected", )).Role(role.StaticText))(ctx)
			// The Interval is 2s as pressing Ctrl+A has a UI performance hit, it will
			// slow down the listing whilst the selection is tallied, want to avoid
			// calling this too often to ensure the data is less tainted.
		}, &testing.PollOptions{Timeout: time.Minute, Interval: 2 * time.Second})
	}

	// showApps := func(ctx context.Context) error {
	// 	return
	// }


	return uiauto.Combine("Navigate to folder, select the file and wait until the total apps count matches expected",
		files.OpenPath(filesapp.FilesTitlePrefix+filesapp.Downloads, filesapp.Downloads, testCase),
		selectFile,
		files.ExpandOpenDropdown(),
		//ui.WaitUntilExists()
	)(ctx)
}
