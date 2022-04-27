// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package telemetryextension

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser/browserfixt"
		"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         APIMemoryInfo2,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Tests chrome.os.telemetry.getMemoryInfo Chrome Extension API function exposed to Telemetry Extension",
		Contacts: []string{
			"lamzin@google.com", // Test and Telemetry Extension author
			"mgawad@google.com", // Telemetry Extension author
			"cros-oem-services-team@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "lacros"},
		Params: []testing.Param{
			{
				Name:              "lacros",
				Fixture:           fixture.LacrosPolicyLoggedIn,
			},
			{
				Name:              "ash",
				Fixture:           fixture.ChromePolicyLoggedIn,
			},
		},
	})
}

// APIMemoryInfo tests chrome.os.telemetry.getMemoryInfo Chrome Extension API functionality.
func APIMemoryInfo2(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	dstURL := "https://chrome.google.com/webstore/detail/chrome-os-diagnostics-com/gogonhoemckpdpadfnjnpgbjpbjnodgc"

	// Reserve 10 seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Connect to Test API.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	// Perform cleanup.
	if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
		s.Fatal("Failed to clean up: ", err)
	}

	// Setup source browser.
	srcBr, closeSrcBrowser, err := browserfixt.SetUp(ctx, s.FixtValue(), browser.TypeLacros)
	if err != nil {
		s.Fatalf("Failed to open the %s source browser: %s", browser.TypeLacros, err)
	}
	defer closeSrcBrowser(cleanupCtx)

	// Ensure google cookies are accepted, it appears when we open the extension link.
	if err := policyutil.EnsureGoogleCookiesAccepted(ctx, srcBr); err != nil {
		s.Fatal("Failed to accept cookies: ", err)
	}

	pwaConn, err := srcBr.NewConn(ctx, "https://www.google.com")
	if err != nil {
		s.Fatal("Failed to create connection to google.com: ", err)
	}
	defer pwaConn.Close()

	if err := chrome.AddTastLibrary(ctx, pwaConn); err != nil {
		s.Fatal("Failed to add Tast library to google.com: ", err)
	}

	if _, err := installTelemetryExtension2(ctx, tconn, srcBr, dstURL); err != nil {
		s.Fatal("Failed to install telem ext: ", err)
	}

	extConn, err := srcBr.NewConn(ctx, "chrome-extension://gogonhoemckpdpadfnjnpgbjpbjnodgc/sw.js")
	if err != nil {
		s.Fatal("Failed to open page: ", err)
	}
	defer extConn.Close()

	if err := chrome.AddTastLibrary(ctx, extConn); err != nil {
		s.Fatal("Failed to add Tast library to Telemetry Extension: ", err)
	}
	var resp interface{}
	if err := extConn.Call(ctx, &resp,
		"tast.promisify(chrome.os.telemetry.getMemoryInfo)",
	); err != nil {
		s.Fatal("Failed to get response from Telemetry extenion service worker: ", err)
	}
}

func installTelemetryExtension2(ctx context.Context, tconn *chrome.TestConn, br *browser.Browser, url string) (bool, error) {
	// Ensure google cookies are accepted, it appears when we open the extension link.
	// if err := policyutil.EnsureGoogleCookiesAccepted(ctx, br); err != nil {
	// 	return false, errors.Wrap(err, "failed to accept cookies")
	// }

	// Open the Chrome Web Store page of the extension.
	conn, err := br.NewConn(ctx, url)
	if err != nil {
		return false, errors.Wrap(err, "failed to connect to chrome")
	}
	defer conn.Close()

	addButton1 := nodewith.Name("Add to Chrome").Role(role.Button).First()
	addButton2 := nodewith.Name("Add extension").Role(role.Button)
	removeButton := nodewith.Name("Remove from Chrome").Role(role.Button).First()

	uia := uiauto.New(tconn)

	if err := uiauto.Combine("Install extension",
		uia.WithTimeout(2 * time.Minute).WaitUntilExists(addButton1),
		uia.WithTimeout(2 * time.Minute).LeftClick(addButton1),
		// The "Add extension" button may not immediately be clickable.
		uia.WithTimeout(2 * time.Minute).LeftClickUntil(addButton2, uia.Gone(addButton2)),
		uia.WithTimeout(2 * time.Minute).WaitUntilExists(removeButton),
	)(ctx); err != nil {
		return false, errors.Wrap(err, "failed to install Telemetry extensions")
	}

	return true, nil
}
