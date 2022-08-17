// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LinkCapturing,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Verifies link capturing integration between ARC and the browser",
		Contacts: []string{
			"tsergeant@chromium.org",
			"chromeos-apps-foundation-team@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
			Fixture:           "arcBooted",
			Val:               browser.TypeAsh,
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Fixture:           "arcBooted",
			Val:               browser.TypeAsh,
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"android_p", "lacros"},
			Fixture:           "lacrosWithArcBooted",
			Val:               browser.TypeLacros,
		}, {
			Name:              "lacros_vm",
			ExtraSoftwareDeps: []string{"android_vm", "lacros"},
			Fixture:           "lacrosWithArcBooted",
			Val:               browser.TypeLacros,
		}},
		Timeout: 2 * time.Minute,
		Data: []string{
			"link_capturing/link_capturing_index.html",
			"link_capturing/app/app_index.html",
		},
	})

}

func LinkCapturing(ctx context.Context, s *testing.State) {
	const (
		serverPort  = 8000
		testApk     = "ArcLinkCapturingTest.apk"
		testPageURL = "http://127.0.0.1:8000/link_capturing/link_capturing_index.html"
	)

	cr := s.FixtValue().(*arc.PreData).Chrome
	arcDevice := s.FixtValue().(*arc.PreData).ARC

	// Give 5 seconds to clean up and dump out UI tree.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// Setup Test API Connection.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	// Setup ARC and install APK.
	if err := arcDevice.WaitIntentHelper(ctx); err != nil {
		s.Fatal("Failed waiting for intent helper: ", err)
	}

	if err := arcDevice.Install(ctx, arc.APKPath(testApk)); err != nil {
		s.Fatal("Failed installing the APK: ", err)
	}

	// Start local server.
	server := &http.Server{Addr: fmt.Sprintf(":%d", 8000), Handler: http.FileServer(s.DataFileSystem())}
	go func() {
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			s.Fatal("Failed to create local server: ", err)
		}
	}()
	defer server.Shutdown(ctx)

	br, closeBrowser, err := browserfixt.SetUp(ctx, cr, s.Param().(browser.Type))
	if err != nil {
		s.Fatal("Failed to open browser: ", err)
	}
	defer closeBrowser(cleanupCtx)

	// Open test page.
	conn, err := br.NewConn(ctx, testPageURL)
	if err != nil {
		s.Fatal("Failed to open test page in browser: ", err)
	}
	defer conn.Close()

	ui := uiauto.New(tconn).WithTimeout(30 * time.Second)

	link := nodewith.Name("In-scope link").Role(role.Link)
	heading := nodewith.Name("In-scope page").Role(role.Heading)

	// Clicking the link should stay in the browser, not open the ARC app.
	if err := uiauto.Combine("Click link to browser",
		ui.LeftClick(link),
		// "In-scope page" text appears on the app_index.html page.
		ui.WaitUntilExists(heading))(ctx); err != nil {
		s.Fatal("Failed to click link: ", err)
	}
}
