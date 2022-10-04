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
		Desc: "Test request mobile site function on websites under different types of log in account",
		Contacts: []string{
			"bob.yang@cienet.com",
			"cienet-development@googlegroups.com",
			"chromeos-sw-engprod@google.com",
		},
		LacrosStatus: testing.LacrosVariantExists,
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Timeout:      12 * time.Minute,
		VarDeps:      []string{"unicorn.childUser", "unicorn.childPassword", "unicorn.parentUser", "unicorn.parentPassword"},
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

const (
	turnOnMobileSite  = checked.True
	turnOffMobileSite = checked.False
)

type mobileTestResources struct {
	user                 userType
	ui                   *uiauto.Context
	threeDotMenuBtn      *nodewith.Finder
	requestMobileSiteBtn *nodewith.Finder
	// cr and outDir are for faillog
	cr     *chrome.Chrome
	outDir string
}

// RequestMobileSiteTablet tests request mobile site function on websites under different types of log in account.
func RequestMobileSiteTablet(ctx context.Context, s *testing.State) {
	credentials := map[userType]chrome.Creds{
		child:  {User: s.RequiredVar("unicorn.childUser"), Pass: s.RequiredVar("unicorn.childPassword"), ParentUser: s.RequiredVar("unicorn.parentUser"), ParentPass: s.RequiredVar("unicorn.parentPassword")},
		normal: {User: s.RequiredVar("unicorn.parentUser"), Pass: s.RequiredVar("unicorn.parentPassword")},
	}
	browserType := s.Param().(browser.Type)

	// TODO(b/244513681): Enable guest mode test for lacros once lacros supports the guest mode.
	if browserType != browser.TypeLacros {
		credentials[guest] = chrome.Creds{}
	}

	websites := map[string]string{
		"Twitter": "https://twitter.com",
		"YouTube": "https://www.youtube.com",
		"Google":  "http://maps.google.com",
	}

	var browserRoot *nodewith.Finder
	if browserType == browser.TypeLacros {
		classNameRegexp := regexp.MustCompile(`^ExoShellSurface(-\d+)?$`)
		browserRoot = nodewith.Role(role.Window).ClassNameRegex(classNameRegexp).NameContaining("Chrome")
	} else {
		browserRoot = nodewith.Role(role.Window).HasClass("BrowserFrame")
	}

	res := &mobileTestResources{
		threeDotMenuBtn:      nodewith.HasClass("BrowserAppMenuButton").Role(role.PopUpButton).Ancestor(nodewith.HasClass("ToolbarView").Ancestor(browserRoot)),
		requestMobileSiteBtn: nodewith.Name("Request mobile site").Role(role.MenuItemCheckBox).Ancestor(nodewith.HasClass("SubmenuView")),
		outDir:               s.OutDir(),
	}

	for user, creds := range credentials {
		f := func(ctx context.Context, s *testing.State) {
			cleanupCtx := ctx
			ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
			defer cancel()

			var opts []chrome.Option
			if creds.User == "" {
				opts = []chrome.Option{chrome.GuestLogin()}
			} else {
				opts = []chrome.Option{chrome.GAIALogin(creds)}
			}

			if browserType == browser.TypeLacros && creds.ParentUser != "" {
				opts = append(opts, chrome.EnableFeatures("LacrosForSupervisedUsers"))
			}

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
				if err := ensureLeftOffSettingOn(ctx, br, res); err != nil {
					s.Fatal("Failed to turn on left off setting: ", err)
				}
			}

			for websiteName, url := range websites {
				if err := mobileSiteTest(ctx, br, res, websiteName, url); err != nil {
					s.Fatalf("Failed to run mobileSiteTest on website %s: %v", websiteName, err)
				}
			}
		}

		if !s.Run(ctx, fmt.Sprintf("request mobile site for %s", user), f) {
			s.Errorf("Failed to run complete test for %s", user)
		}
	}
}

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
		conn.CloseTarget(ctx)
		conn.Close()
	}(cleanupCtx)

	testing.ContextLog(ctx, "Waiting web page achieve quiescence")
	if err := webutil.WaitForQuiescence(ctx, conn, time.Minute); err != nil {
		return errors.Wrap(err, "failed to wait for web page achieve quiescence")
	}

	testing.ContextLog(ctx, `Turning on "request mobile site"`)
	if err := requestMobileSiteAndVerify(ctx, conn, websiteName, res, turnOnMobileSite); err != nil {
		return errors.Wrap(err, "failed to turn on request mobile site")
	}

	testing.ContextLog(ctx, `Turning off "request mobile site"`)
	if err := requestMobileSiteAndVerify(ctx, conn, websiteName, res, turnOffMobileSite); err != nil {
		return errors.Wrap(err, "failed to turn off request mobile site")
	}

	testing.ContextLog(ctx, `Turning on "request mobile site"`)
	if err := requestMobileSiteAndVerify(ctx, conn, websiteName, res, turnOnMobileSite); err != nil {
		return errors.Wrap(err, "failed to turn on request mobile site")
	}

	testing.ContextLogf(ctx, `Re-visiting website %q`, url)
	if err := reVisitWeb(ctx, conn, url); err != nil {
		return errors.Wrapf(err, "failed to revisit web site %s", url)
	}

	if err := verifyMobileSite(ctx, res, turnOnMobileSite); err != nil {
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
// verifies if the 'request mobile site button status' is same as expected status
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

// ensureLeftOffSettingOn opens chrome://settings/onStartup and verifies if 'continue where you left off' is on,
// turns 'continue where you left off' on if not on.
func ensureLeftOffSettingOn(ctx context.Context, br *browser.Browser, res *mobileTestResources) (retErr error) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	starpupPage := "chrome://settings/onStartup"
	conn, err := br.NewConn(ctx, starpupPage)
	if err != nil {
		return errors.Wrapf(err, "failed to open %s", starpupPage)
	}
	defer func(ctx context.Context) {
		faillog.DumpUITreeWithScreenshotOnError(ctx, res.outDir, func() bool { return retErr != nil }, res.cr, fmt.Sprintf("ui_dump_left_off_%s", res.user))
		conn.CloseTarget(ctx)
		conn.Close()
	}(cleanupCtx)

	coninueLeftOff := nodewith.Name("Continue where you left off")
	return uiauto.Combine(`turn on "continue where you left off"`,
		res.ui.LeftClick(coninueLeftOff.Role(role.InlineTextBox)),
		res.ui.WaitUntilExists(coninueLeftOff.Role(role.RadioButton)),
	)(ctx)
}
