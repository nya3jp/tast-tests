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
			Val: false,
		}, {
			Name: "swa",
			Val:  true,
		}},
	})
}

func UIPerf(ctx context.Context, s *testing.State) {
	swaEnabled := s.Param().(bool)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	var chromeOpts []chrome.Option
	filesLauncher := filesapp.Launch
	if swaEnabled {
		chromeOpts = append(chromeOpts, chrome.EnableFilesAppSWA())
		filesLauncher = filesapp.LaunchSWA
	}

	cr, err := chrome.New(ctx, chromeOpts...)
	if err != nil {
		s.Fatal("Cannot start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	files, err := filesLauncher(ctx, tconn)
	if err != nil {
		s.Fatal("Launching the Files App failed: ", err)
	}

	ew, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Error creating keyboard: ", err)
	}
	defer ew.Close()

	pv := perf.NewValues()

	// Measure how long 100 and 1,000 files take to render in Files app listbox.
	for _, totalFiles := range []int{100, 1000} {
		testCase := fmt.Sprintf("%d_files", totalFiles)
		s.Run(ctx, testCase, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, testCase)

			duration := testDirectoryListing(ctx, s, files, ew, testCase, totalFiles)

			pv.Set(perf.Metric{
				Name:      testCase,
				Unit:      "ms",
				Direction: perf.SmallerIsBetter,
			}, duration)
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

func testDirectoryListing(ctx context.Context, s *testing.State, files *filesapp.FilesApp, ew *input.KeyboardEventWriter, testCase string, totalFiles int) float64 {
	folderPath := filepath.Join(filesapp.DownloadPath, testCase)
	if err := os.MkdirAll(folderPath, 0644); err != nil {
		s.Fatalf("Failed to create directory %q: %v", folderPath, err)
	}
	defer os.RemoveAll(folderPath)
	if err := createDirectoryListing(ctx, folderPath, totalFiles); err != nil {
		s.Fatalf("Failed to create folder with name %q: %v", folderPath, err)
	}

	var startTime time.Time
	filesAllLoaded := func() uiauto.Action {
		return func(ctx context.Context) error {
			startTime = time.Now()
			return testing.Poll(ctx, func(ctx context.Context) error {
				if err := ew.Accel(ctx, "ctrl+A"); err != nil {
					s.Fatal("Failed selecting files with Ctrl+A: ", err)
				}
				return files.Exists(nodewith.Name(fmt.Sprintf("%d files selected", totalFiles)).Role(role.StaticText))(ctx)
			}, &testing.PollOptions{Timeout: time.Minute})
		}
	}

	// Wait until cpu idle before starting performance measures.
	if err := cpu.WaitUntilIdle(ctx); err != nil {
		s.Fatal("Failed to wait: ", err)
	}

	if err := uiauto.Combine("Navigate to folder and keep selecting all until total count matches expected",
		files.OpenPath(filesapp.FilesTitlePrefix+filesapp.Downloads, filesapp.Downloads, testCase),
		filesAllLoaded(),
	)(ctx); err != nil {
		s.Fatal("Failed to navigate to folder: ", err)
	}

	return float64(time.Since(startTime).Milliseconds())
}
