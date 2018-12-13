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

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChromeExtension,
		Desc:         "Demonstrates loading a custom Chrome extension",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome_login"},
	})
}

func ChromeExtension(ctx context.Context, s *testing.State) {
	extDir, err := ioutil.TempDir("/tmp", "tast.example.ChromeExtension.")
	if err != nil {
		s.Fatal("Failed to create temp dir: ", err)
	}
	defer os.RemoveAll(extDir)

	s.Log("Writing unpacked extension to ", extDir)
	const manifest = `{
  "key": "MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQDmxClP0X5BDqNOvQ7G9tagwuk61pPpyoj1xHNLhTS30T272iJhUJ1YyCD2QRp4GkNgorWEc5KNFBMWq7l0fkvM9mJEvrlJaiWWUIASFOhII1ImpetFNjDlQ4hm97Dz6P+fymIFNLlJ6UyPkITBLFeDHwOYEraCu64+FFmKaRueVQIDAQAB",
  "description": "Demonstration extension",
  "name": "Demo extension",
  "background": { "scripts": ["background.js"] },
  "manifest_version": 2,
  "version": "0.1",
  "permissions": ["system.memory"]
}`
	for _, f := range []struct{ name, data string }{
		{"manifest.json", manifest},
		{"background.js", ""},
	} {
		if err = ioutil.WriteFile(filepath.Join(extDir, f.name), []byte(f.data), 0644); err != nil {
			s.Fatalf("Failed to write %v: %v", f.name, err)
		}
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
	if err := conn.WaitForExpr(ctx, "'memory' in chrome.system"); err != nil {
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
