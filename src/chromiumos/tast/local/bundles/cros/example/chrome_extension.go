// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"context"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"

	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChromeExtension,
		Desc:         "Demonstrates loading a custom Chrome extension",
		Contacts:     []string{"derat@chromium.org", "tast-users@chromium.org"},
		Data:         []string{"chrome_extension_manifest.json"},
		SoftwareDeps: []string{"chrome_login"},
	})
}

func ChromeExtension(ctx context.Context, s *testing.State) {
	extDir, err := ioutil.TempDir("", "tast.example.ChromeExtension.")
	if err != nil {
		s.Fatal("Failed to create temp dir: ", err)
	}
	defer os.RemoveAll(extDir)

	// Please use the shared test extension (see chrome.Chrome.TestAPIConn) whenever possible,
	// adding additional permissions to its manifest file if needed.
	// Loading your own extension is only required in special cases, e.g. if you need to use
	// the clipboardRead and clipboardWrite permissions to interact with a background page.
	s.Log("Writing unpacked extension to ", extDir)
	if err := fsutil.CopyFile(s.DataPath("chrome_extension_manifest.json"),
		filepath.Join(extDir, "manifest.json")); err != nil {
		s.Fatal("Failed to copy manifest: ", err)
	}
	if err := ioutil.WriteFile(filepath.Join(extDir, "background.js"), []byte{}, 0644); err != nil {
		s.Fatal("Failed to write background.js: ", err)
	}

	extID, err := chrome.ComputeExtensionID(extDir)
	if err != nil {
		s.Fatalf("Failed to compute extension ID for %v: %v", extDir, err)
	}
	s.Log("Extension ID is ", extID)

	cr, err := chrome.New(ctx, chrome.UnpackedExtension(extDir))
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

	s.Log("Connecting to background page")
	bgURL := chrome.ExtensionBackgroundPageURL(extID)
	conn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(bgURL))
	if err != nil {
		s.Fatalf("Failed to connect to background page at %v: %v", bgURL, err)
	}
	defer conn.Close()

	// APIs are not immediately available to extensions: https://crbug.com/789313
	s.Log("Waiting for chrome.system.memory API to become available")
	if err := conn.WaitForExpr(ctx, "chrome.system.memory"); err != nil {
		s.Fatal("chrome.system.memory API unavailable: ", err)
	}

	s.Log("Exercising chrome.system.memory API")
	var info struct {
		Capacity          float64 `json:"capacity"`
		AvailableCapacity float64 `json:"availableCapacity"`
	}
	if err := conn.EvalPromise(ctx,
		`new Promise((resolve, reject) => {
			chrome.system.memory.getInfo((info) => { resolve(info); });
		})`, &info); err != nil {
		s.Fatal("Failed to call chrome.system.memory.getInfo: ", err)
	}
	mb := func(bytes float64) int { return int(math.Round(bytes / (1024 * 1024))) }
	s.Logf("System has %d MB available of %d MB total", mb(info.AvailableCapacity), mb(info.Capacity))
}
