// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package filemanager

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/cpu"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

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
		Params: []testing.Param{{
			Val: false, // Use the Chrome app version of Files app.
		}, {
			Name: "swa",
			Val:  true, // Use the System Web App version of Files app.
		}},
	})
}

func UIPerf(ctx context.Context, s *testing.State) {
	swaEnabled := s.Param().(bool)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Enable or disable the SWA based on supplied params.
	var chromeOpts []chrome.Option
	filesLauncher := filesapp.Launch
	if swaEnabled {
		chromeOpts = append(chromeOpts, chrome.EnableFilesAppSWA())
		filesLauncher = filesapp.LaunchSWA
	}

	// Start Chrome with SWA flag.
	cr, err := chrome.New(ctx, chromeOpts...)
	if err != nil {
		s.Fatal("Cannot start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	// Get Test API connection.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	// Launch the Files app (SWA or Chrome app depends on Params).
	files, err := filesLauncher(ctx, tconn)
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

	// Measure how long 100 and 1,000 files take to render in Files app listbox.
	for _, totalFiles := range []int{100, 1000} {
		testCase := fmt.Sprintf("%d_files", totalFiles)
		uma := fmt.Sprintf("FileBrowser.DirectoryListLoad.my_files.%d", totalFiles)
		s.Run(ctx, testCase, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, testCase)

			// Run the directory listing test and observe the histogram for output.
			histograms, err := metrics.Run(ctx, tconn, func(ctx context.Context) error {
				return testDirectoryListing(ctx, s, files, ew, testCase, totalFiles)
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

			pv.Set(perf.Metric{
				Name:      testCase,
				Unit:      "ms",
				Direction: perf.SmallerIsBetter,
			}, float64(histograms[0].Sum))
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
		filePath := filepath.Join(folderPath, fmt.Sprintf("File-%d.file", i))
		if err := ioutil.WriteFile(filePath, []byte("blah"), 0644); err != nil {
			return errors.Wrapf(err, "failed to create file with path %q", filePath)
		}
	}
	return nil
}

func testDirectoryListing(ctx context.Context, s *testing.State, files *filesapp.FilesApp, ew *input.KeyboardEventWriter, testCase string, totalFiles int) error {
	folderPath := filepath.Join(filesapp.DownloadPath, testCase)
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

	filesAllLoaded := func() uiauto.Action {
		return func(ctx context.Context) error {
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
	}

	return uiauto.Combine("Navigate to folder and keep selecting all until total count matches expected",
		files.OpenPath(filesapp.FilesTitlePrefix+filesapp.Downloads, filesapp.Downloads, testCase),
		filesAllLoaded(),
	)(ctx)
}
