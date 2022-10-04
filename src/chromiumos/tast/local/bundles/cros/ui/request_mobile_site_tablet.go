// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/checked"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: RequestMobileSiteTablet,
		Desc: "Test request mobile site function on websites under different types of login account",
		Contacts: []string{
			"cj.tsai@cienet.com",
			"cienet-development@googlegroups.com",
			"chromeos-sw-engprod@google.com",
		},
		LacrosStatus: testing.LacrosVariantExists,
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Timeout:      12 * time.Minute,
		VarDeps: []string{
			"family.parentEmail",
			"family.parentPassword",
			"family.unicornEmail",
			"family.unicornPassword",
		},
		Params: []testing.Param{
			{
				Val: browser.TypeAsh,
			},
			{
				Name:              "lacros",
				ExtraSoftwareDeps: []string{"lacros"},
				Val:               browser.TypeLacros,
			},
		},
	})
}

type userType string

const (
	guest  userType = "guest_user"
	normal userType = "normal_user"
	child  userType = "child_user"
)

type mobileTestResources struct {
	user                 userType
	ui                   *uiauto.Context
	threeDotMenuBtn      *nodewith.Finder
	requestMobileSiteBtn *nodewith.Finder
	// cr is for faillog.
	cr *chrome.Chrome
	// outDir is for faillog.
	outDir string
}

func RequestMobileSiteTablet(ctx context.Context, s *testing.State) {
	optsForUser := map[userType][]chrome.Option{
		child: []chrome.Option{
			chrome.GAIALogin(chrome.Creds{
				User:       s.RequiredVar("family.unicornEmail"),
				Pass:       s.RequiredVar("family.unicornPassword"),
				ParentUser: s.RequiredVar("family.parentEmail"),
				ParentPass: s.RequiredVar("family.parentPassword"),
			}),
		},
		normal: []chrome.Option{
			chrome.GAIALogin(chrome.Creds{
				User: s.RequiredVar("family.parentEmail"),
				Pass: s.RequiredVar("family.parentPassword"),
			}),
		},
	}

	browserType := s.Param().(browser.Type)
	switch browserType {
	case browser.TypeAsh:
		// TODO(b/244513681): Enable guest mode test for lacros once lacros supports the guest mode.
		optsForUser[guest] = []chrome.Option{chrome.GuestLogin()}
	case browser.TypeLacros:
		optsForUser[child] = append(optsForUser[child], chrome.EnableFeatures("LacrosForSupervisedUsers"))
	}

	websites := map[string]string{
		"Twitter": "https://twitter.com",
		"YouTube": "https://www.youtube.com",
		"Google":  "http://maps.google.com",
	}

	browserRoot := nodewith.Role(role.Window).HasClass("BrowserFrame")
	if browserType == browser.TypeLacros {
		browserRoot = nodewith.Role(role.Window).ClassNameRegex(regexp.MustCompile(`^ExoShellSurface(-\d+)?$`)).NameContaining("Chrome")
	}

	res := &mobileTestResources{
		threeDotMenuBtn:      nodewith.HasClass("BrowserAppMenuButton").Role(role.PopUpButton).Ancestor(nodewith.HasClass("ToolbarView").Ancestor(browserRoot)),
		requestMobileSiteBtn: nodewith.Name("Request mobile site").Role(role.MenuItemCheckBox).Ancestor(nodewith.HasClass("SubmenuView")),
		outDir:               s.OutDir(),
	}

	for user, opts := range optsForUser {
		s.Run(ctx, fmt.Sprintf("request mobile site for %s", user), func(ctx context.Context, s *testing.State) {
			cleanupCtx := ctx
			ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
			defer cancel()

			s.Log("Logging in as ", user)
			cr, br, closeBrowser, err := browserfixt.SetUpWithNewChrome(ctx, browserType, lacrosfixt.NewConfig(), opts...)
			if err != nil {
				s.Fatal("Failed to sign in: ", err)
			}
			defer cr.Close(cleanupCtx)
			defer closeBrowser(cleanupCtx)

			tconn, err := cr.TestAPIConn(ctx)
			if err != nil {
				s.Fatal("Failed to get Test API connection: ", err)
			}

			res.user = user
			res.ui = uiauto.New(tconn)
			res.cr = cr

			cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, true)
			if err != nil {
				s.Fatal("Failed to enable the tablet mode: ", err)
			}
			defer cleanup(cleanupCtx)

			// Guest has no left off setting.
			if user != guest {
				if err := ensureLeftOffSettingEnabled(ctx, br, res); err != nil {
					s.Fatal("Failed to turn on left off setting: ", err)
				}
			}

			for websiteName, url := range websites {
				if err := mobileSiteTest(ctx, br, res, websiteName, url); err != nil {
					s.Fatalf("Failed to run mobileSiteTest on website %q: %v", websiteName, err)
				}
			}
		})
	}
}

// mobileSiteTest verifies the expected mobile site status when "request mobile site" is on and off,
// then revisits the website to verify that "request mobile site" is still on.
func mobileSiteTest(ctx context.Context, br *browser.Browser, res *mobileTestResources, websiteName, url string) (retErr error) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	conn, err := br.NewConn(ctx, url)
	if err != nil {
		return errors.Wrapf(err, "failed to open page %q", url)
	}
	defer func(ctx context.Context) {
		faillog.DumpUITreeWithScreenshotOnError(ctx, res.outDir, func() bool { return retErr != nil }, res.cr, fmt.Sprintf("ui_dump_%s", res.user))
		if err := conn.CloseTarget(ctx); err != nil {
			testing.ContextLogf(ctx, "Failed to close website %q: %v", url, err)
		}
		if err := conn.Close(); err != nil {
			testing.ContextLogf(ctx, "Failed to close test connection to website %q: %v", url, err)
		}
	}(cleanupCtx)

	testing.ContextLog(ctx, "Waiting for website to achieve quiescence")
	if err := webutil.WaitForQuiescence(ctx, conn, time.Minute); err != nil {
		return errors.Wrap(err, "failed to wait for website to achieve quiescence")
	}

	testing.ContextLog(ctx, `Turning on "request mobile site"`)
	if err := requestMobileSiteAndVerify(ctx, conn, websiteName, res, checked.True); err != nil {
		return errors.Wrap(err, `failed to turn on "request mobile site"`)
	}

	testing.ContextLog(ctx, `Turning off "request mobile site"`)
	if err := requestMobileSiteAndVerify(ctx, conn, websiteName, res, checked.False); err != nil {
		return errors.Wrap(err, `failed to turn off "request mobile site"`)
	}

	testing.ContextLog(ctx, `Turning on "request mobile site"`)
	if err := requestMobileSiteAndVerify(ctx, conn, websiteName, res, checked.True); err != nil {
		return errors.Wrap(err, `failed to turn on "request mobile site"`)
	}

	testing.ContextLogf(ctx, "Revisiting website %q", url)
	if err := conn.Navigate(ctx, url); err != nil {
		return errors.Wrapf(err, "failed to navigate to %q", url)
	}

	if err := verifyMobileSite(ctx, res, checked.True); err != nil {
		return errors.Wrap(err, `failed to verify "request mobile site"`)
	}
	return nil
}

// requestMobileSiteAndVerify opens the three dot menu, clicks the "request mobile site"
// button, waits for the website to be stable, and verifies the expected mobile site status.
func requestMobileSiteAndVerify(ctx context.Context, conn *chrome.Conn, websiteName string, res *mobileTestResources, status checked.Checked) error {
	if err := uiauto.Combine(`select "request mobile site"`,
		res.ui.LeftClick(res.threeDotMenuBtn),
		res.ui.LeftClick(res.requestMobileSiteBtn),
		webutil.WaitForQuiescenceAction(conn, time.Minute),
	)(ctx); err != nil {
		return err
	}

	if err := verifyMobileSite(ctx, res, status); err != nil {
		return errors.Wrap(err, "failed to verify mobile site status")
	}
	return nil
}

// verifyMobileSite opens the three dot menu, checks if the "request mobile site" button is checked,
// verifies the expected "request mobile site" button status, and then closes the three dot menu.
func verifyMobileSite(ctx context.Context, res *mobileTestResources, expected checked.Checked) error {
	if err := uiauto.Combine(`open "request mobile site" option`,
		res.ui.LeftClick(res.threeDotMenuBtn),
		res.ui.WaitUntilExists(res.requestMobileSiteBtn),
	)(ctx); err != nil {
		return err
	}

	if nodeInfo, err := res.ui.Info(ctx, res.requestMobileSiteBtn); err != nil {
		return errors.Wrap(err, "failed to get node information")
	} else if nodeInfo.Checked != expected {
		return errors.Errorf("failed to verify mobile site status; got: %v, want: %v", nodeInfo.Checked, expected)
	}

	return uiauto.Combine(`close "three dot menu"`,
		res.ui.LeftClick(res.threeDotMenuBtn),
		res.ui.WaitUntilGone(nodewith.HasClass("SubmenuView").Role(role.Menu)),
	)(ctx)
}

// ensureLeftOffSettingEnabled opens chrome://settings/onStartup, ensures that "continue
// where you left off" is turned on, and then closes chrome://settings/onStartup.
func ensureLeftOffSettingEnabled(ctx context.Context, br *browser.Browser, res *mobileTestResources) (retErr error) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	const startupPage = "chrome://settings/onStartup"
	conn, err := br.NewConn(ctx, startupPage)
	if err != nil {
		return errors.Wrapf(err, "failed to open %q", startupPage)
	}
	defer func(ctx context.Context) {
		faillog.DumpUITreeWithScreenshotOnError(ctx, res.outDir, func() bool { return retErr != nil }, res.cr, fmt.Sprintf("ui_dump_left_off_%s", res.user))
		if err := conn.CloseTarget(ctx); err != nil {
			testing.ContextLogf(ctx, "Failed to close website %q: %v", startupPage, err)
		}
		if err := conn.Close(); err != nil {
			testing.ContextLogf(ctx, "Failed to close test connection to website %q: %v", startupPage, err)
		}
	}(cleanupCtx)

	continueLeftOff := nodewith.Name("Continue where you left off").Role(role.RadioButton)
	verify := res.ui.WaitUntilExists(continueLeftOff.Attribute("checked", "true"))
	return uiauto.IfFailThen(
		verify,
		uiauto.Combine(`turn on "continue where you left off"`,
			res.ui.LeftClick(continueLeftOff),
			verify,
		),
	)(ctx)
}
