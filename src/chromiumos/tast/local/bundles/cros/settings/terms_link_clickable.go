// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package settings

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TermsLinkClickable,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks the terms of service link is clickable within help page",
		Contacts: []string{
			"cienet-development@googlegroups.com",
			"chromeos-sw-engprod@google.com",
			"ting.chen@cienet.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Fixture: "chromeLoggedIn",
			Val:     browser.TypeAsh,
		}, {
			Name:              "lacros",
			Fixture:           "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Val:               browser.TypeLacros,
		}},
	})
}

// TermsLinkClickable checks the chrome://terms link is clickable within 'About ChromeOS' and chrome://help.
func TermsLinkClickable(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}
	// Setup a browser.
	bt := s.Param().(browser.Type)
	br, closeBrowser, err := browserfixt.SetUp(ctx, cr, bt)
	if err != nil {
		s.Fatal("Failed to open the browser: ", err)
	}
	defer closeBrowser(cleanupCtx)

	webPageTest := func(ctx context.Context, s *testing.State) {
		conn, err := br.NewConn(ctx, "chrome://help")
		if err != nil {
			s.Fatal("Failed to connect to chrome: ", err)
		}
		defer conn.Close()
		defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "web_page_dump")

		if err := checkTermsOfService(ctx, cr, br, tconn, s.OutDir()); err != nil {
			s.Fatalf("Failed to open terms in %v browser from web page: %v", bt, err)
		}
	}

	ossettingsTest := func(ctx context.Context, s *testing.State) {
		settings, err := ossettings.LaunchAtPage(ctx, tconn, ossettings.AboutChromeOS)
		if err != nil {
			s.Fatal("Failed to launch OS settings at `About ChromeOS` page: ", err)
		}
		defer settings.Close(cleanupCtx)
		defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ossettings_dump")

		if err := checkTermsOfService(ctx, cr, br, tconn, s.OutDir()); err != nil {
			s.Fatalf("Failed to open terms in %v browser from settings page: %v", bt, err)
		}
	}

	for _, subtest := range []struct {
		name string
		f    func(ctx context.Context, s *testing.State)
	}{
		{"check web page", webPageTest},
		{"check os-settings", ossettingsTest},
	} {
		if !s.Run(ctx, subtest.name, subtest.f) {
			s.Errorf("Failed to run subtest %s", subtest.name)
		}
	}
}

func checkTermsOfService(ctx context.Context, cr *chrome.Chrome, br *browser.Browser, tconn *chrome.TestConn, outDir string) error {
	ui := uiauto.New(tconn)

	termsOfServiceLink := nodewith.Name("Terms of Service").Role(role.Link)
	if err := uiauto.Combine("click terms of service link",
		ui.FocusAndWait(termsOfServiceLink),
		ui.WaitUntilExists(termsOfServiceLink),
		ui.LeftClick(termsOfServiceLink),
	)(ctx); err != nil {
		return err
	}

	return verifyContent(ctx, cr, br, outDir)
}

func verifyContent(ctx context.Context, cr *chrome.Chrome, br *browser.Browser, outDir string) (err error) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	url := "chrome://terms/"
	conn, err := br.NewConnForTarget(ctx, chrome.MatchTargetURL(url))
	if err != nil {
		return errors.Wrapf(err, "failed to connect to window %s", url)
	}
	defer conn.Close()
	defer conn.CloseTarget(cleanupCtx)
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, outDir, func() bool { return err != nil }, cr, "terms_dump")

	// Verify the content is within the terms page.
	expected := "Google Chrome and ChromeOS Additional Terms of Service"
	expr := fmt.Sprintf(`document.querySelector('h2').innerText === '%s'`, expected)
	if err := conn.WaitForExprWithTimeout(ctx, expr, 10*time.Second); err != nil {
		return errors.Wrap(err, "unexpected page content")
	}
	return nil
}
