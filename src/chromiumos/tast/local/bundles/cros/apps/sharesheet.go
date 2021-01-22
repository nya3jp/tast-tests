// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package apps

import (
	"bytes"
	"context"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/local/chrome/ui/sharesheet"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Sharesheet,
		Desc: "Verify sharing a file to PWA works",
		Contacts: []string{
			"benreich@chromium.org",
			"chromeos-apps-foundation-team@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		Data:         []string{"sharesheet_manifest.json", "sharesheet_service.js", "sharesheet_index.html", "sharesheet_icon.png"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
	})
}

func Sharesheet(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	const (
		appShareLabel        = "Web Share Target Test App"
		expectedFileName     = "test.txt"
		expectedFileContents = "test file contents"
		pwaIndexPage         = "http://localhost:8080/sharesheet_index.html"
		installTimeout       = 15 * time.Second
	)

	// Setup the test file.
	testFileLocation := filepath.Join(filesapp.DownloadPath, expectedFileName)
	if err := ioutil.WriteFile(testFileLocation, []byte(expectedFileContents), 0644); err != nil {
		s.Fatalf("Creating file %s failed: %s", testFileLocation, err)
	}
	defer os.Remove(testFileLocation)

	fileChan := make(chan string)

	fs := http.FileServer(s.DataFileSystem())
	http.Handle("/", fs)
	http.HandleFunc("/web-share-target/", func(w http.ResponseWriter, r *http.Request) {
		parseErr := r.ParseMultipartForm(4096)
		if parseErr != nil {
			s.Fatal("Failed parsing multipart form: ", parseErr)
			return
		}

		receivedFile := r.MultipartForm.File["received_file"][0]
		fileChan <- receivedFile.Filename

		multipartFile, err := receivedFile.Open()
		if err != nil {
			s.Fatal("Failed reading the received file: ", err)
		}

		buf := new(bytes.Buffer)
		buf.ReadFrom(multipartFile)

		fileChan <- buf.String()
	})

	go func() {
		if err := http.ListenAndServe(":8080", nil); err != nil {
			s.Fatal("Failed creating local server")
		}
	}()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	appID, err := apps.InstallPWAForURL(ctx, cr, pwaIndexPage, installTimeout)
	if err != nil {
		s.Fatal("Failed installing PWA for URL: ", err)
	}

	if err := ash.WaitForChromeAppInstalled(ctx, tconn, appID, installTimeout); err != nil {
		s.Fatal("Failed waiting for PWA to be installed: ", err)
	}

	// Open the Files App.
	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed launching the Files app: ", err)
	}
	defer files.Release(ctx)

	// Open the Downloads folder and check for the test file.
	if err := files.OpenDownloads(ctx); err != nil {
		s.Fatal("Failed opening the Downloads folder: ", err)
	}

	if err := files.SelectContextMenu(ctx, expectedFileName, filesapp.Share); err != nil {
		s.Fatal("Failed to click share button in context menu: ", err)
	}

	if err := sharesheet.ClickApp(ctx, tconn, appShareLabel); err != nil {
		s.Fatal("Failed clicking app on stable sharesheet: ", err)
	}

	fileName := <-fileChan
	fileContents := <-fileChan

	if fileContents != expectedFileContents {
		s.Fatalf("File contents shared did not match: got %q; want %q", fileContents, expectedFileContents)
	}

	if fileName != expectedFileName {
		s.Fatalf("File name shared did not match: got %q; want %q", fileName, expectedFileName)
	}
}
