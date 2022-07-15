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
	"chromiumos/tast/local/chrome/uiauto/vkb"
	"chromiumos/tast/local/chrome/useractions"
	"chromiumos/tast/local/kioskmode"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardKioskTyping,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks that user can type in virtual keyboard in kiosk mode",
		Contacts:     []string{"jhtin@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		SearchFlags:  util.IMESearchFlags([]ime.InputMethod{ime.DefaultInputMethod}),
		Timeout:      2 * time.Minute,
		Params: []testing.Param{
			{
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

func VirtualKeyboardKioskTyping(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 15*time.Second)
	defer cancel()

	// Launches the server for e14s-test page.
	server := testserver.LaunchServer(ctx)
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
	opts := []chrome.Option{chrome.ExtraArgs("--force-tablet-mode=touch_view"), chrome.VKEnabled()}
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

	userContext, err := inputactions.NewInputsUserContextWithoutState(ctx, "", s.OutDir(), cr, tconn, nil)
	if err != nil {
		s.Fatal("Failed to create inputs user context: ", err)
	}

	vkbCtx := vkb.NewContext(cr, tconn)
	defer vkbCtx.HideVirtualKeyboard()(ctx)

	ui := uiauto.New(tconn)
	inputField := testserver.TextAreaInputField

	actionName := "VK typing in inputfield"
	if err := uiauto.UserAction(actionName,
		uiauto.Combine(actionName,
			ui.WaitUntilExists(inputField.Finder()),
			ui.MakeVisible(inputField.Finder()),
			vkbCtx.ClickUntilVKShown(inputField.Finder()),
			vkbCtx.TapKeysIgnoringCase(strings.Split("abcdefghijklmnopqrstuvwxyz", "")),
			util.WaitForFieldTextToBeIgnoringCase(tconn, inputField.Finder(), "abcdefghijklmnopqrstuvwxyz"),
		),
		userContext,
		&useractions.UserActionCfg{
			Attributes: map[string]string{
				useractions.AttributeFeature: useractions.FeatureVKTyping,
			},
		},
	)(ctx); err != nil {
		s.Fatal("Failed to validate VK typing: ", err)
	}
}
