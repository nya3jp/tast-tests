// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
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
		Desc: "Test request mobile site function on websites under different types of log in account",
		Contacts: []string{
			"bob.yang@cienet.com",
			"cienet-development@googlegroups.com",
			"chromeos-sw-engprod@google.com",
		},
		LacrosStatus: testing.LacrosVariantNeeded,
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Timeout:      12 * time.Minute,
		VarDeps:      []string{"unicorn.childUser", "unicorn.childPassword", "unicorn.parentUser", "unicorn.parentPassword"}})
}

type userType string

const (
	guest  userType = "guest user"
	normal userType = "normal user"
	child  userType = "child user"
)

const (
	turnOnMmobileSite  = checked.True
	turnOffMmobileSite = checked.False
)

type mobileTestResources struct {
	ui                   *uiauto.Context
	threeDotMenuBtn      *nodewith.Finder
	requestMobileSiteBtn *nodewith.Finder
}

//RequestMobileSiteTablet test request mobile site function on websites under different types of log in account.
func RequestMobileSiteTablet(ctx context.Context, s *testing.State) {
	accounts := map[userType]chrome.Creds{
		child:  {User: s.RequiredVar("unicorn.childUser"), Pass: s.RequiredVar("unicorn.childPassword"), ParentUser: s.RequiredVar("unicorn.parentUser"), ParentPass: s.RequiredVar("unicorn.parentPassword")},
		normal: {User: s.RequiredVar("unicorn.parentUser"), Pass: s.RequiredVar("unicorn.parentPassword")},
		guest:  {},
	}

	websites := map[string]string{
		"Twitter": "https://twitter.com",
		"YouTube": "https://www.youtube.com",
		"Google":  "http://maps.google.com",
		"Amazon":  "http://amazon.com",
		"CNN":     "https://edition.cnn.com/",
	}

	for usr, creds := range accounts {
		f := func(ctx context.Context, s *testing.State) {
			cleanupCtx := ctx
			ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
			defer cancel()

			testing.ContextLog(ctx, "Logging in as ", usr)
			cr, tconn, err := signIn(ctx, creds)
			if err != nil {
				s.Fatal("Failed to sign in: ", err)
			}
			defer cr.Close(cleanupCtx)

			res := &mobileTestResources{
				ui:                   uiauto.New(tconn),
				threeDotMenuBtn:      nodewith.HasClass("BrowserAppMenuButton").Role(role.PopUpButton).Ancestor(nodewith.HasClass("ToolbarView")),
				requestMobileSiteBtn: nodewith.Name("Request mobile site").Role(role.MenuItemCheckBox).Ancestor(nodewith.HasClass("SubmenuView")),
			}

			cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, true)
			if err != nil {
				s.Fatal("Failed to enable the tablet mode: ", err)
			}
			defer cleanup(cleanupCtx)

			// Guest has no left off setting.
			if usr != guest {
				if err := ensureLeftOffSettingOn(ctx, cr, res); err != nil {
					s.Fatal("Failed to turn on left off setting failed: ", err)
				}
			}

			for websiteName, url := range websites {
				if err := mobileSiteTest(ctx, cr, res, websiteName, url, s.OutDir()); err != nil {
					s.Fatalf("Failed to run mobileSiteTest on website %s: %v", websiteName, err)
				}
			}
		}

		if !s.Run(ctx, fmt.Sprintf("reqest mobile site for %s", usr), f) {
			s.Fatalf("Failed to run complete test for %s", usr)
		}
	}
}

func mobileSiteTest(ctx context.Context, cr *chrome.Chrome, res *mobileTestResources, websiteName, url, outDir string) (retErr error) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	conn, err := cr.NewConn(ctx, url)
	if err != nil {
		return errors.Wrapf(err, "failed to open page %q", url)
	}
	defer func(ctx context.Context) {
		faillog.DumpUITreeWithScreenshotOnError(ctx, outDir, func() bool { return retErr != nil }, cr, "ui_dump")
		conn.CloseTarget(ctx)
		conn.Close()
	}(cleanupCtx)

	testing.ContextLog(ctx, "Waiting web page achieve quiescence")
	if err := webutil.WaitForQuiescence(ctx, conn, time.Minute); err != nil {
		return errors.Wrap(err, "failed to wait for web page achieve quiescence")
	}

	testing.ContextLog(ctx, `Turning on "request mobile site"`)
	if err := requestMobileSiteAndVerify(ctx, conn, websiteName, res, turnOnMmobileSite); err != nil {
		return errors.Wrap(err, "failed to turn on request mobile site")
	}

	testing.ContextLog(ctx, `Turning off "request mobile site"`)
	if err := requestMobileSiteAndVerify(ctx, conn, websiteName, res, turnOffMmobileSite); err != nil {
		return errors.Wrap(err, "failed to turn off request mobile site")
	}

	testing.ContextLog(ctx, `Turning on "request mobile site"`)
	if err := requestMobileSiteAndVerify(ctx, conn, websiteName, res, turnOnMmobileSite); err != nil {
		return errors.Wrap(err, "failed to turn on request mobile site")
	}

	testing.ContextLogf(ctx, `Re-visiting website %q`, url)
	if err := reVisitWeb(ctx, conn, url); err != nil {
		return errors.Wrapf(err, "failed to revisit web site %s", url)
	}

	if err := verifyMobileSite(ctx, res, turnOnMmobileSite); err != nil {
		return errors.Wrap(err, "failed to verify request mobile site")
	}
	return nil
}

// requestMobileSiteAndVerify opens three dot menu and click request mobile site button, waits for web page to be stable
// and then verify if mobile site status is same as expected.
func requestMobileSiteAndVerify(ctx context.Context, conn *chrome.Conn, websiteName string, res *mobileTestResources, status checked.Checked) error {
	if err := uiauto.Combine(`select "request mobile site"`,
		res.ui.LeftClick(res.threeDotMenuBtn),
		res.ui.LeftClick(res.requestMobileSiteBtn),
	)(ctx); err != nil {
		return err
	}
	testing.ContextLog(ctx, "Waiting web page achieve quiescence")
	if err := webutil.WaitForQuiescence(ctx, conn, time.Minute); err != nil {
		return errors.Wrap(err, "failed to wait for web page achieve quiescence")
	}

	if err := verifyMobileSite(ctx, res, status); err != nil {
		return errors.Wrap(err, "failed to verify mobile site status")
	}
	return nil
}

// verifyMobileSite opens three dot menu and check if request mobile site button is checked,
// verify if the 'request mobile site button status' is same as expected status
// and closes three dot menu after verification.
func verifyMobileSite(ctx context.Context, res *mobileTestResources, expected checked.Checked) error {
	if err := uiauto.Combine(`open "request mobile site" option`,
		res.ui.LeftClick(res.threeDotMenuBtn),
		res.ui.WaitUntilExists(res.requestMobileSiteBtn),
	)(ctx); err != nil {
		return err
	}

	nodeInfo, err := res.ui.Info(ctx, res.requestMobileSiteBtn)
	if err != nil {
		return errors.Wrap(err, "failed to get node information")
	}

	if nodeInfo.Checked != expected {
		return errors.Wrapf(err, "failed to verify mobile site status, want: %v, got: %v", expected, nodeInfo.Checked)
	}

	if err := res.ui.LeftClick(res.threeDotMenuBtn)(ctx); err != nil {
		return errors.Wrap(err, "failed to click threeDotMenuButton to close menu")
	}
	return nil
}

// reVisitWeb re-visit the same website by calling conn.Navigate, then verify if request mobile site is on.
func reVisitWeb(ctx context.Context, conn *chrome.Conn, url string) (err error) {
	if err := conn.Navigate(ctx, url); err != nil {
		return errors.Wrapf(err, "failed to navigate to %s: ", url)
	}
	return nil
}

// ensureLeftOffSettingOn opens chrome://settings/onStartup and verify if "continue where you left off" is on
// turn 'continue where you left off on if not' on.
func ensureLeftOffSettingOn(ctx context.Context, cr *chrome.Chrome, res *mobileTestResources) error {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	starpupPage := "chrome://settings/onStartup"
	conn, err := cr.NewConn(ctx, starpupPage)
	if err != nil {
		return errors.Wrapf(err, "failed to open %s", starpupPage)
	}
	defer conn.Close()
	defer conn.CloseTarget(cleanupCtx)

	coninueLeftOff := nodewith.Name("Continue where you left off")
	return uiauto.Combine(`turn on "continue where you left off"`,
		res.ui.LeftClick(coninueLeftOff.Role(role.InlineTextBox)),
		res.ui.WaitUntilExists(coninueLeftOff.Role(role.RadioButton)),
	)(ctx)
}

// signIn logs in chrome os with chrome.Creds.
func signIn(ctx context.Context, creds chrome.Creds) (cr *chrome.Chrome, tconn *chrome.TestConn, retErr error) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	var opts []chrome.Option
	if creds.User == "" {
		opts = []chrome.Option{chrome.GuestLogin()}
	} else {
		opts = []chrome.Option{chrome.GAIALogin(creds)}
	}

	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to login to Chrome")
	}
	defer func(ctx context.Context) {
		if retErr != nil {
			cr.Close(ctx)
		}
	}(cleanupCtx)

	tconn, err = cr.TestAPIConn(ctx)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to get Test API connection")
	}

	return cr, tconn, nil
}
