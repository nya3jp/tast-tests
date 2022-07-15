// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/inputs/inputactions"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/bundles/cros/inputs/util"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/useractions"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/kioskmode"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         KioskPhysicalKeyboardCapsLock,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks that user can lock Caps on physical keyboard",
		Contacts:     []string{"shengjun@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		SearchFlags:  util.IMESearchFlags([]ime.InputMethod{ime.DefaultInputMethod}),
		Timeout:      2 * time.Minute,
		Params: []testing.Param{
			{
				Name:      "ash",
				Fixture:   fixture.KioskAutoLaunchCleanup,
				ExtraAttr: []string{"informational", "group:input-tools-upstream"},
				Val:       ime.EnglishUS,
			},
			{
				Name:      "lacros",
				Fixture:   fixture.KioskAutoLaunchCleanup,
				ExtraAttr: []string{"informational", "group:input-tools-upstream"},
				Val:       ime.EnglishUS,
			},
		},
	})
}

func KioskPhysicalKeyboardCapsLock(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 15*time.Second)
	defer cancel()

	// LaunchBrowser doesn't work because browser process cannot be found in kiosk mode.
	server, err := testserver.LaunchServer(ctx)

	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()
	// WebKioskAccountID identifier of the web Kiosk application.
	webKioskAccountID := "arbitrary_id_web_kiosk_1@managedchrome.com"
	webKioskAccountType := policy.AccountTypeKioskWebApp
	webKioskIconURL := server.URL + "/e14s-test"
	webKioskTitle := "TastKioskModeSetByPolicyE14sPage"
	webKioskURL := server.URL + "/e14s-test"
	// DeviceLocalAccountInfo uses *string instead of string for internal data
	// structure. That is needed since fields in json are marked as omitempty.
	webKioskPolicy := policy.DeviceLocalAccountInfo{
		AccountID:   &webKioskAccountID,
		AccountType: &webKioskAccountType,
		WebKioskAppInfo: &policy.WebKioskAppInfo{
			Url:     &webKioskURL,
			Title:   &webKioskTitle,
			IconUrl: &webKioskIconURL,
		}}

	// KioskAppAccountID identifier of the Kiosk application.
	KioskAppAccountID := "arbitrary_id_store_app_2@managedchrome.com"
	kioskAppAccountType := policy.AccountTypeKioskApp
	// KioskAppID pointing to the Printtest app - not listed in the WebStore.
	KioskAppID := "aajgmlihcokkalfjbangebcffdoanjfo"
	kioskAppPolicy := policy.DeviceLocalAccountInfo{
		AccountID:   &KioskAppAccountID,
		AccountType: &kioskAppAccountType,
		KioskAppInfo: &policy.KioskAppInfo{
			AppId: &KioskAppID,
		}}

	// DefaultLocalAccountsConfiguration holds default Kiosks accounts
	// configuration. Each, when setting public account policies can be
	// referred by id: KioskAppAccountID and WebKioskAccountID
	localAccountsConfiguration := &policy.DeviceLocalAccounts{
		Val: []policy.DeviceLocalAccountInfo{
			kioskAppPolicy,
			webKioskPolicy,
		},
	}

	var opts []chrome.Option
	if strings.Contains(s.TestName(), "lacros") {
		opts = append(opts, chrome.ExtraArgs("--enable-features=LacrosSupport,WebKioskEnableLacros", "--lacros-availability-ignore"))
	}
	kiosk, cr, err := kioskmode.New(
		ctx,
		fdms,
		kioskmode.AutoLaunch(kioskmode.WebKioskAccountID),
		kioskmode.CustomLocalAccounts(localAccountsConfiguration),
		kioskmode.ExtraChromeOptions(opts...),
	)
	if err != nil {
		s.Fatal("Failed to start Chrome in Kiosk mode: ", err)
	}
	defer kiosk.Close(ctx)
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree")

	// required since kioskfixture doesn't automatically provide these
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get Test API connection: ", err)
	}
	uc, err := inputactions.NewInputsUserContextWithoutState(ctx, "", s.OutDir(), cr, tconn, nil)
	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer keyboard.Close()

	// CloseAll seems to crash due to its.closeBrowser(ctx) failing due to NPE. Close() doens't closeBrowser.
	//	defer its.Close()
	defer server.Close()
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	inputField := testserver.TextAreaInputField
	// Pressing the shift key by itself does not have any affect if no caps lock.
	// However if the test failed while caps lock is still enabled, shift will
	// disable it. This is just to make sure the state of the caps lock is clean
	// for the next test that uses the fixture.
	defer keyboard.AccelAction("Shift")(cleanupCtx)

	ui := uiauto.New(tconn)
	// Even document is ready, target is not yet in a11y tree.
	if err := ui.WaitUntilExists(testserver.PageRootFinder)(ctx); err != nil {
		s.Fatal("Failed to render page: ", err)
	}

	actionName := "PK enable caps lock, type and disable caps lock"
	if err := uiauto.UserAction(actionName,
		uiauto.Combine(actionName,
			keyboard.AccelAction("Alt+Search"),
			ui.MakeVisible(inputField.Finder()),
			ui.LeftClick(inputField.Finder()),
			keyboard.TypeAction("abcdefghijklmnopqrstuvwxyz01234! ABCDEFGHIJKLMNOPQRSTUVWXYZ01234!"),
			util.WaitForFieldTextToBe(tconn, inputField.Finder(), "ABCDEFGHIJKLMNOPQRSTUVWXYZ01234! abcdefghijklmnopqrstuvwxyz01234!"),
			keyboard.AccelAction("Shift"),
		),
		uc,
		&useractions.UserActionCfg{
			Attributes: map[string]string{
				useractions.AttributeFeature: useractions.FeaturePKTyping,
			},
		},
	)(ctx); err != nil {
		s.Fatal("Failed to validate caps lock: ", err)
	}
}
