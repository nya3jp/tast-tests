// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/testing"
)

type debugUITreeDumpTestParams struct {
	browserType browser.Type
	getLocation bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         DebugUITreeDump,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "For debugging an issue where UI tree dumps do not seem right",
		Contacts:     []string{"amusbach@chromium.org", "xiyuan@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"debug_ui_tree_dump.html"},
		Params: []testing.Param{{
			Name:    "get_location",
			Val:     debugUITreeDumpTestParams{browserType: browser.TypeAsh, getLocation: true},
			Fixture: "chromeLoggedIn",
		}, {
			Name:              "get_location_lacros",
			Val:               debugUITreeDumpTestParams{browserType: browser.TypeLacros, getLocation: true},
			Fixture:           "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
		}, {
			Name:    "just_wait",
			Val:     debugUITreeDumpTestParams{browserType: browser.TypeAsh, getLocation: false},
			Fixture: "chromeLoggedIn",
		}, {
			Name:              "just_wait_lacros",
			Val:               debugUITreeDumpTestParams{browserType: browser.TypeLacros, getLocation: false},
			Fixture:           "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
		}},
	})
}

func DebugUITreeDump(ctx context.Context, s *testing.State) {
	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	params := s.Param().(debugUITreeDumpTestParams)
	cr, l, cs, err := lacros.Setup(ctx, s.FixtValue(), params.browserType)
	if err != nil {
		s.Fatal("Failed to initialize test: ", err)
	}
	defer lacros.CloseLacros(cleanupCtx, l)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	srv := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer srv.Close()

	conn, err := cs.NewConn(ctx, srv.URL+"/debug_ui_tree_dump.html")
	if err != nil {
		s.Fatal("Failed to load debug_ui_tree_dump.html: ", err)
	}
	defer conn.Close()

	if err := webutil.WaitForQuiescence(ctx, conn, 10*time.Second); err != nil {
		s.Fatal("Failed to wait for debug_ui_tree_dump.html to achieve quiescence: ", err)
	}

	if params.getLocation {
		startTime := time.Now()
		bounds, err := uiauto.New(tconn).Location(ctx, nodewith.Name("shuddering bookcases with gingerbread heads").Role(role.Button))
		if err != nil {
			s.Fatal("Failed to get shuddering bookcases button bounds: ", err)
		}
		s.Logf("Took %v to get shuddering bookcases button bounds %v", time.Since(startTime), bounds)
	} else {
		if err := testing.Sleep(ctx, 10*time.Second); err != nil {
			s.Fatal("Failed to sleep: ", err)
		}
	}

	if err := uiauto.LogRootDebugInfo(ctx, tconn, filepath.Join(s.OutDir(), "ui_tree.txt")); err != nil {
		s.Fatal("Failed to dump UI tree: ", err)
	}
}
