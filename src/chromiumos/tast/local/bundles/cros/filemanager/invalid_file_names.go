// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package filemanager

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: InvalidFileNames,
		Desc: "Basic smoke test for the Files app",
		Contacts: []string{
			"chromeos-files-syd@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
		Timeout: 50*time.Minute,
	})
}

func InvalidFileNames(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	for i := 122; i < 1024; i++ { 
		// Setup the test file.
		
		si, err := strconv.Unquote(`"\u` + fmt.Sprintf("%04d", i) + `"`)
		textFile := "test" + si + ".txt"
		s.Logf("Attempting to open %s %s", textFile, si, i)
		
		testFileLocation := filepath.Join(filesapp.DownloadPath, textFile)
		if err := ioutil.WriteFile(testFileLocation, []byte("blahblah"), 0644); err != nil {
			s.Fatalf("Creating file %s failed: %s", testFileLocation, err)
		}
		defer os.Remove(testFileLocation)

		// Open the test API.
		tconn, err := cr.TestAPIConn(ctx)
		if err != nil {
			s.Fatal("Creating test API connection failed: ", err)
		}
		defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

		// Open the Files App.
		files, err := filesapp.Launch(ctx, tconn)
		if err != nil {
			s.Fatal("Launching the Files App failed: ", err)
		}

		// Define keyboard to perform keyboard shortcuts.
		ew, err := input.Keyboard(ctx)
		if err != nil {
			s.Fatal("Error creating keyboard: ", err)
		}
		defer ew.Close()
		
		if err := uiauto.Combine("open downloads and check items",
			// Open the Downloads folder and check for the test file.
			files.OpenDownloads(),
			files.WaitForFile(textFile),
			files.SelectFile(textFile),
			ew.AccelAction("Enter"))(ctx); err != nil {
			s.Fatal("Failed to smoke test the Files App: ", err)
		}

		// Get connection to foreground extension to verify changes.
		dropTargetURL := "file:///"
		conn, err := cr.NewConnForTarget(ctx, func(t *chrome.Target) bool {
			s.Log(t.URL)
			return strings.HasPrefix(t.URL, dropTargetURL)
		})
		if err != nil {
			s.Fatalf("Could not connect to extension at %v: %v", dropTargetURL, err)
		}
		defer conn.Close()

		testing.Sleep(ctx, time.Second)
	
		// Make sure the extension title has loaded JavaScript properly.
		if err := conn.WaitForExprFailOnErrWithTimeout(ctx, "document.body.innerText == 'blahblah'", 5*time.Second); err != nil {
			s.Fatal("Failed waiting for javascript to update window.document.title: ", err)
		}

		testing.Sleep(ctx, time.Second)
		
		if err := conn.CloseTarget(ctx); err != nil {
			s.Fatal("Failed to close target associated with file")
		}
	}
}
