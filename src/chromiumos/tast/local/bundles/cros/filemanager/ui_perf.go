// Copyright 2022 The ChromiumOS Authors
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
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/cpu"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/filemanager"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

type perfTestFunc = func(ctx context.Context, s *testing.State, files *filesapp.FilesApp, ew *input.KeyboardEventWriter, testCase, downloadsPath string, testSize int) error

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
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "UI performance metrics for Files app",
		Contacts: []string{
			"benreich@chromium.org",
			"lucmult@chromium.org",
			"chromeos-files-syd@google.com",
		},
		Timeout:      4 * time.Minute,
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		Params: []testing.Param{
			{
				Name: "directory_list",
				Val: perfTest{
					sizes:       []int{100, 1000},
					testFunc:    testDirectoryListing,
					umaName:     "FileBrowser.DirectoryListLoad.my_files.%d",
					installPwas: false,
				},
				Fixture: "openFilesApp",
			},
			{
				Name: "list_apps",
				Val: perfTest{
					sizes:       []int{10}, // The metric is 10 apps.
					testFunc:    testAvailableAppsListing,
					umaName:     "FileBrowser.UpdateAvailableApps.%d",
					installPwas: true,
				},
				Fixture: "install10Pwas",
			},
		},
	})
}

func UIPerf(ctx context.Context, s *testing.State) {
	fixt := s.FixtValue().(filemanager.FixtureData)
	params := s.Param().(perfTest)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cr := fixt.Chrome

	downloadsPath, err := cryptohome.DownloadsPath(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to retrieve users Downloads path: ", err)
	}

	// Get Test API connection.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	files := fixt.FilesWindow

	// Get a keyboard handle to use accelerators such as Ctrl+A.
	ew, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Error creating keyboard: ", err)
	}
	defer ew.Close()

	pv := perf.NewValues()

	// Measure each "size" iteration of the test.
	for _, size := range params.sizes {
		testCase := fmt.Sprintf("%d_size", size)
		s.Logf("Running test: %s", testCase)
		uma := fmt.Sprintf(params.umaName, size)
		s.Run(ctx, testCase, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, testCase)

			// Run the directory listing test and observe the histogram for output.
			histograms, err := metrics.Run(ctx, tconn, func(ctx context.Context) error {
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
	const totalFiles = 100
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
			return errors.Errorf("waiting for at least %d apps, got %d ", totalApps, len(nodes))
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
