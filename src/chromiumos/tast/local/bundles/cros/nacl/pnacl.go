// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package nacl

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"

	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/testing"
)

var extensionFiles = []string{
	"chrome_nacl_app/background.js",
	"chrome_nacl_app/manifest.json",
	"chrome_nacl_app/nacl_module.nmf",
	"chrome_nacl_app/nacl_module.pexe",
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Pnacl,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Tests running a PNaCl module",
		Contacts:     []string{"emaxx@chromium.org", "nacl-eng@google.com"},
		Data:         extensionFiles,
		SoftwareDeps: []string{"chrome", "nacl"},
		Attr:         []string{"group:mainline"},
		Params: []testing.Param{{
			Val: browser.TypeAsh,
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			ExtraAttr:         []string{"informational"},
			Val:               browser.TypeLacros,
		},
		},
	})
}

func Pnacl(ctx context.Context, s *testing.State) {
	extDir, err := ioutil.TempDir("", "tast.nacl.PnaclApp.")
	if err != nil {
		s.Fatal("Failed to create temp dir: ", err)
	}
	defer os.RemoveAll(extDir)

	for _, file := range extensionFiles {
		dst := filepath.Join(extDir, filepath.Base(file))
		if err := fsutil.CopyFile(s.DataPath(file), dst); err != nil {
			s.Fatalf("Failed to copy %q file to %q: %v", file, extDir, err)
		}
	}

	extID, err := chrome.ComputeExtensionID(extDir)
	if err != nil {
		s.Fatalf("Failed to compute extension ID for %v: %v", extDir, err)
	}

	var extOpt chrome.Option
	bt := s.Param().(browser.Type)
	switch bt {
	case browser.TypeLacros:
		extOpt = chrome.LacrosUnpackedExtension(extDir)
	case browser.TypeAsh:
		extOpt = chrome.UnpackedExtension(extDir)
	}
	cr, br, closeBrowser, err := browserfixt.SetUpWithNewChrome(ctx, bt, lacrosfixt.NewConfig(), extOpt)

	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer closeBrowser(ctx)
	defer cr.Close(ctx)

	s.Log("Connecting to background page")
	bgURL := chrome.ExtensionBackgroundPageURL(extID)
	conn, err := br.NewConnForTarget(ctx, chrome.MatchTargetURL(bgURL))
	if err != nil {
		s.Fatalf("Failed to connect to background page at %v: %v", bgURL, err)
	}
	defer conn.Close()

	s.Log("Waiting for JS test function to become available")
	if err := conn.WaitForExpr(ctx, "runTest"); err != nil {
		s.Fatal("JS test function unavailable: ", err)
	}

	s.Log("Executing JS test function")
	if err := conn.Eval(ctx, "runTest()", nil); err != nil {
		s.Fatal("Failed to call JS test function: ", err)
	}
}
