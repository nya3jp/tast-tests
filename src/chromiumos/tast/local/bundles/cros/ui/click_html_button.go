// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/lacros"
	"chromiumos/tast/local/lacros/launcher"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ClickHTMLButton,
		Desc:         "For discussion only; intended to exist only as a CL and never in the code base",
		Contacts:     []string{"amusbach@chromium.org", "jonfan@chromium.org"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"html_button.html"},
		Params: []testing.Param{{
			Fixture: "chromeLoggedIn",
			Val:     lacros.ChromeTypeChromeOS,
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			ExtraData:         []string{launcher.DataArtifact},
			Fixture:           "lacrosStartedByData",
			Val:               lacros.ChromeTypeLacros,
		}},
	})
}

func ClickHTMLButton(ctx context.Context, s *testing.State) {
	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// TODO(crbug.com/1127165): Remove the artifactPath argument when we can use Data in fixtures.
	var artifactPath string
	if s.Param().(lacros.ChromeType) == lacros.ChromeTypeLacros {
		artifactPath = s.DataPath(launcher.DataArtifact)
	}
	cr, l, cs, err := lacros.Setup(ctx, s.FixtValue(), artifactPath, s.Param().(lacros.ChromeType))
	if err != nil {
		s.Fatal("Failed to initialize test: ", err)
	}
	defer lacros.CloseLacrosChrome(cleanupCtx, l)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	srv := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer srv.Close()

	conn, err := cs.NewConn(ctx, srv.URL+"/html_button.html")
	if err != nil {
		s.Fatal("Failed to load html_button.html: ", err)
	}
	defer conn.Close()

	if err := webutil.WaitForQuiescence(ctx, conn, 10*time.Second); err != nil {
		s.Fatal("Failed to wait for html_button.html to achieve quiescence: ", err)
	}

	if err := uiauto.New(tconn).LeftClick(nodewith.Name("Squeegee").Role(role.Button))(ctx); err != nil {
		s.Fatal("Failed to click the Squeegee button: ", err)
	}

	if err := testing.Sleep(ctx, 10*time.Second); err != nil {
		s.Fatal("Failed to wait ten seconds: ", err)
	}
}
