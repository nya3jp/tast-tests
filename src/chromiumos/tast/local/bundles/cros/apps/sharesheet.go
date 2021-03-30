// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package apps

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
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

type fileNameAndContents struct {
	name     string
	contents string
}

func Sharesheet(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	const (
		appShareLabel        = "Web Share Target Test App"
		expectedFileName     = "test.txt"
		expectedFileContents = "test file contents"
		localServerPort      = 8080
		installTimeout       = 15 * time.Second
	)

	// Setup the test file.
	testFileLocation := filepath.Join(filesapp.DownloadPath, expectedFileName)
	if err := ioutil.WriteFile(testFileLocation, []byte(expectedFileContents), 0644); err != nil {
		s.Fatalf("Failed to create file %q: %s", testFileLocation, err)
	}
	defer os.Remove(testFileLocation)

	fileChan := make(chan fileNameAndContents)
	defer close(fileChan)

	mux := http.NewServeMux()
	fs := http.FileServer(s.DataFileSystem())
	mux.Handle("/", fs)
	mux.HandleFunc("/web-share-target/", func(w http.ResponseWriter, r *http.Request) {
		if parseErr := r.ParseMultipartForm(4096); parseErr != nil {
			s.Fatal("Failed to parse multipart form: ", parseErr)
			return
		}

		receivedFile := r.MultipartForm.File["received_file"][0]
		multipartFile, err := receivedFile.Open()
		if err != nil {
			s.Fatal("Failed to read the received file: ", err)
		}

		buf := new(bytes.Buffer)
		buf.ReadFrom(multipartFile)

		fileChan <- fileNameAndContents{name: receivedFile.Filename, contents: buf.String()}
	})

	server := &http.Server{Addr: fmt.Sprintf(":%v", localServerPort), Handler: mux}
	go func() {
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			s.Fatal("Failed to create local server: ", err)
		}
	}()
	defer server.Shutdown(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	appID, err := apps.InstallPWAForURL(ctx, cr, fmt.Sprintf("http://localhost:%v/sharesheet_index.html", localServerPort), installTimeout)
	if err != nil {
		s.Fatal("Failed to install PWA for URL: ", err)
	}

	if err := ash.WaitForChromeAppInstalled(ctx, tconn, appID, installTimeout); err != nil {
		s.Fatal("Failed to wait for PWA to be installed: ", err)
	}

	// Open the Files App.
	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch the Files app: ", err)
	}

	// The Sharesheet appears to not properly update the accessibility tree with
	// the coordinates whilst animating. The total time to animate is currently 150ms
	// so setting to 1s to ensure low-end devices are given enough time.
	sharesheet := uiauto.New(tconn).WithInterval(time.Second)
	sharesheetTargetButton := nodewith.Role(role.Button).NameContaining(appShareLabel).ClassName("SharesheetTargetButton")

	if err := uiauto.Combine("Open Downloads and Click sharesheet",
		files.OpenDownloads(),
		files.ClickContextMenuItem(expectedFileName, filesapp.Share),
		sharesheet.LeftClick(sharesheetTargetButton),
	)(ctx); err != nil {
		s.Fatal("Failed to open downloads and click share button: ", err)
	}

	var receivedFile fileNameAndContents
	select {
	case receivedFile = <-fileChan:
	case <-ctx.Done():
		s.Fatal("Timeout waiting to receive shared file name and contents")
	}

	if receivedFile.name != expectedFileName {
		s.Errorf("File name shared did not match: got %q; want %q", receivedFile.name, expectedFileName)
	}

	if receivedFile.contents != expectedFileContents {
		s.Errorf("File contents shared did not match: got %q; want %q", receivedFile.contents, expectedFileContents)
	}
}
