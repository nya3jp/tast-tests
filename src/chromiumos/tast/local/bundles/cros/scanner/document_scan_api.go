// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package scanner

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DocumentScanAPI,
		Desc:         "Check that DocumentScan API works", //TODO
		Contacts:     []string{"kmoed@google.com", "project-bolton@google.com"},
		Data:         []string{"manifest.json", "background.js", "scan.css", "scan.html", "scan.js"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
	})

}

// DocumentScanAPI tests if chrome.documentScan API is working.
func DocumentScanAPI(ctx context.Context, s *testing.State) {
	// Use cleanupCtx for any deferred cleanups in case of timeouts or
	// cancellations on the shortened context.
	//cleanupCtx := ctx  TODO
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	//Set up virtual printer with image
	//TODO: get all of process from scan_escl_ipp.go

	//Following code example from drag_drop.go
	extDir, err := ioutil.TempDir("", "tast.scanner.DocumentScanAPI.")
	if err != nil {
		s.Fatal("Failed to create temp extemsopm dir: ", err)
	}
	defer os.RemoveAll(extDir)

	scanTargetExtID, err := setupDocumentScanExtension(ctx, s, extDir)
	if err != nil {
		s.Fatal("Failed setup of document scan extension: ", err)
	}
	s.Log("Document Scan Extension ID is ", scanTargetExtID)

	cr, err := chrome.New(ctx, chrome.UnpackedExtension(extDir))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	// Open the test API (TODO: necessary?)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API con failed: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// s.Log("Connecting to background page")
	// bgURL := chrome.ExtensionBackgroundPageURL(scanTargetExtID)
	// conn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(bgURL))
	// if err != nil {
	// 	s.Fatalf("Failed to connect to background page at %v: %v", bgURL, err)
	// }
	// defer conn.Close()

	// content, err := conn.PageContent(ctx)
	// if err != nil {
	// 	s.Fatalf("Failed to get page content: ", err)
	// }
	// s.Log("Content: ", content)

	fgURL := "chrome-extension://" + scanTargetExtID + "/scan.html"
	conn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(fgURL))
	if err != nil {
		s.Fatalf("Failed to connect to foreground page at %v: %v", fgURL, err)
	}
	defer conn.Close()
	// err = conn.Navigate(ctx, "chrome-extension://"+scanTargetExtID+"/scan.html")
	// if err != nil {
	// 	s.Fatalf("Failed to nav: ", err)
	// }
	// testing.Sleep(ctx, 3600*time.Second)

	// APIs are not immediately available to extensions: https://crbug.com/789313
	s.Log("Waiting for chrome.documentScan API to become available")
	if err := conn.WaitForExpr(ctx, "chrome.documentScan"); err != nil {
		s.Fatal("chrome.documentScan API unavailable: ", err)
	}

	// var fgURL = "chrome-extension://" + scanTargetExtID + "/scan.html"
	// if err := conn.EvalPromise(ctx,
	// 	`new Promise((resolve, reject) => {
	// 		window.open((fgURL) => { resolve(fgURL); });
	// 	})`, &fgURL); err != nil {
	// 	s.Fatal("Failed to open window: ", err)
	// }

	//Click on scan button -- TODO: make into function?
	// var button chrome.JSObject
	// //get the button
	// if err := conn.Eval(ctx, "document.getElementById('scanButton')", &button); err != nil {
	// 	s.Fatal("Failed to get scan button from app: ", err)
	// }
	//create new node to click on button?

	//Validate scan worked

}

// setupDocumentScanExtension moves the extension files into th eextension directory and returns extension ID.
func setupDocumentScanExtension(ctx context.Context, s *testing.State, extDir string) (string, error) {
	for _, name := range []string{"manifest.json", "background.js",
		"scan.html", "scan.js"} {
		if err := fsutil.CopyFile(s.DataPath(name), filepath.Join(extDir, name)); err != nil {
			return "", errors.Wrapf(err, "failed to copy extension %q: %v", name, err)
		}
	}

	extID, err := chrome.ComputeExtensionID(extDir)
	if err != nil {
		s.Fatalf("Failed to compute extension ID for %q: %v", extDir, err)
	}

	return extID, nil
}

// // AppInfo contains the information necessary to load the test app.
// type AppInfo struct {
// 	appDocument		string
// 	appWindowID		string
// }

// // TODO: desc
// func LaunchTestApp(ctx context.Context, appInfo* AppInfo) {

// 	cmd := "chrome.app.window.create('%s', {
// 			singleton: true,
// 			id: '%s',
// 			state: 'fullscreen'
// 		});" , appInfo.appDocument, appInfo.appWindowID

// }

// // TODO: desc
// func DocumentScanAPI(ctx context.Context, s *testing.State) {
// 	// Use cleanupCtx for any deferred cleanups in case of timeouts or
// 	// cancellations on the shortened context.
// 	//cleanupCtx := ctx  TODO
// 	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
// 	defer cancel()

// 	//Set up virtual printer
// 	//TODO: get all of process from scan_escl_ipp.go

// 	//Following code example from drag_drop.go
// 	extDir, err := ioutil.TempDir("", "tast.scanner.DocumentScanAPI.")
// 	if err != nil {
// 		s.Fatal("Failed to create temp extemsopm dir: ", err)
// 	}
// 	defer os.RemoveAll(extDir)

// 	// Please use the shared test extension (see chrome.Chrome.TestAPIConn) whenever possible,
// 	// adding additional permissions to its manifest file if needed.
// 	// Loading your own extension is only required in special cases, e.g. if you need to use
// 	// the clipboardRead and clipboardWrite permissions to interact with a background page.
// 	s.Log("Writing unpacked extension to ", extDir)
// 	if err := fsutil.CopyFile(s.DataPath("chrome_extension_manifest.json"),
// 		filepath.Join(extDir, "manifest.json")); err != nil {
// 		s.Fatal("Failed to copy manifest: ", err)
// 	}
// 	if err := ioutil.WriteFile(filepath.Join(extDir, "background.js"), []byte{}, 0644); err != nil {
// 		s.Fatal("Failed to write background.js: ", err)
// 	}

// 	extID, err := chrome.ComputeExtensionID(extDir)
// 	if err != nil {
// 		s.Fatalf("Failed to compute extension ID for %v: %v", extDir, err)
// 	}
// 	s.Log("Extension ID is ", extID)

// 	cr, err := chrome.New(ctx, chrome.UnpackedExtension(extDir))
// 	if err != nil {
// 		s.Fatal("Chrome login failed: ", err)
// 	}
// 	defer cr.Close(ctx)

// 	s.Log("Connecting to background page")
// 	bgURL := chrome.ExtensionBackgroundPageURL(extID)
// 	conn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(bgURL))
// 	if err != nil {
// 		s.Fatalf("Failed to connect to background page at %v: %v", bgURL, err)
// 	}
// 	defer conn.Close()

// 	// APIs are not immediately available to extensions: https://crbug.com/789313
// 	s.Log("Waiting for chrome.documentScan API to become available")
// 	if err := conn.WaitForExpr(ctx, "chrome.documentScan"); err != nil {
// 		s.Fatal("chrome.documentScan API unavailable: ", err)
// 	}

// 	// Info determined by documentscan_AppTestWithFakeLorgnette.py.
// 	// app AppInfo {
// 	// 	appDocument:		"scan.html"
// 	// 	appWindowID:		"ChromeApps-Sample-Document-Scan"
// 	// }
// 	// LaunchTestApp(ctx, &AppInfo)
// 	//chrome.management.LaunchApp()
// 	//TODO: TEST = taken from test 3
// 	//params := ui.FindParams(Role: ui.RoleTypeButton)

// 	// faillog.saveScreenshotCDP(ctx, s.OutDir())
// 	// s.Fatal("I would like a screenshot")
// 	// s.Log("out dir: ")
// 	// s.Log(s.OutDir())
// 	// tconn, err := cr.TestAPIConn(ctx)
// 	// if err != nil {
// 	// 	s.Fatal("Failed to create Test API connection: ", err)
// 	// }
// 	// ////////////////////"chromiumos/tast/local/chrome/ui/faillog"
// 	// defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)
// 	// s.Fatal("I would like a UI dump")
// 	//Generate virtual mouse device
// 	// s.Log("finding and opening mouse device")
// 	// mew, err := input.Mouse(ctx)
// 	// if err != nil {
// 	// 	s.Fatal("Failed to open mouse device: ", err)
// 	// }
// 	// defer mew.Close()
// 	//use mouse click	- find bounds of display/item location
// 	//mew.Click()  //clicks and releases the mouse
// 	//to start scan
// 	//verify scan
// }
