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
		Func:         KioskPhysicalKeyboard,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks that user can lock Caps on physical keyboard",
		Contacts:     []string{"jhtin@chromium.org", "essential-inputs-team@google.com"},
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

func KioskPhysicalKeyboard(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 15*time.Second)
	defer cancel()

	// Launches the server for e14s-test page.
	server, err := testserver.LaunchServer(ctx)
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer server.Close()

	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Creating a kiosk mode configuration that will launch an app that points to the e14s-test page.
	webKioskAccountID := "arbitrary_id_web_kiosk_1@managedchrome.com"
	webKioskAccountType := policy.AccountTypeKioskWebApp
	webKioskTitle := "TastKioskModeSetByPolicyE14sPage"
	webKioskURL := server.URL + "/e14s-test"
	webKioskPolicy := policy.DeviceLocalAccountInfo{
		AccountID:   &webKioskAccountID,
		AccountType: &webKioskAccountType,
		WebKioskAppInfo: &policy.WebKioskAppInfo{
			Url:     &webKioskURL,
			Title:   &webKioskTitle,
			IconUrl: &webKioskURL,
		}}

	kioskAppAccountID := "arbitrary_id_store_app_2@managedchrome.com"
	kioskAppAccountType := policy.AccountTypeKioskApp
	kioskAppPolicy := policy.DeviceLocalAccountInfo{
		AccountID:   &kioskAppAccountID,
		AccountType: &kioskAppAccountType,
		KioskAppInfo: &policy.KioskAppInfo{
			AppId: &kioskmode.KioskAppID,
		}}

	localAccountsConfiguration := &policy.DeviceLocalAccounts{
		Val: []policy.DeviceLocalAccountInfo{
			kioskAppPolicy,
			webKioskPolicy,
		},
	}

	// If lacros launch kiosk app in lacros mode.
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

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get Test API connection: ", err)
	}

	uc, err := inputactions.NewInputsUserContextWithoutState(ctx, "", s.OutDir(), cr, tconn, nil)
	if err != nil {
		s.Fatal("Failed to create inputs user context: ", err)
	}

	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer keyboard.Close()

	ui := uiauto.New(tconn)
	inputField := testserver.TextAreaInputField
	if err := ui.WaitUntilExists(inputField.Finder())(ctx); err != nil {
		s.Fatal("Failed to render page: ", err)
	}

	actionName := "PK enable caps lock, type and disable caps lock"
	if err := uiauto.UserAction(actionName,
		uiauto.Combine(actionName,
			ui.MakeVisible(inputField.Finder()),
			ui.LeftClick(inputField.Finder()),
			keyboard.TypeAction("abcdefghijklmnopqrstuvwxyz01234! ABCDEFGHIJKLMNOPQRSTUVWXYZ01234!"),
			util.WaitForFieldTextToBe(tconn, inputField.Finder(), "abcdefghijklmnopqrstuvwxyz01234! ABCDEFGHIJKLMNOPQRSTUVWXYZ01234!"),
		),
		uc,
		&useractions.UserActionCfg{
			Attributes: map[string]string{
				useractions.AttributeFeature: useractions.FeaturePKTyping,
			},
		},
	)(ctx); err != nil {
		s.Fatal("Failed to validate caps lock: ", err)
		info, _ := uiauto.RootDebugInfo(ctx, tconn)
		s.Log(ctx, info)
	}
}
