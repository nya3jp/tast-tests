// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast-tests/local/bundles/cros/inputs/inputactions"
	"chromiumos/tast-tests/local/bundles/cros/inputs/pre"
	"chromiumos/tast-tests/local/chrome"
	"chromiumos/tast-tests/local/chrome/uiauto"
	"chromiumos/tast-tests/local/chrome/uiauto/faillog"
	"chromiumos/tast-tests/local/chrome/uiauto/nodewith"
	"chromiumos/tast-tests/local/chrome/uiauto/role"
	"chromiumos/tast-tests/local/chrome/uiauto/vkb"
	"chromiumos/tast-tests/local/chrome/useractions"
	"chromiumos/tast-tests/local/uidetection"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardLoginScreen,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that the virtual keyboard works on login screen",
		Attr:         []string{"group:mainline", "group:input-tools", "informational"},
		Contacts:     []string{"shengjun@google.com", "essential-inputs-team@google.com"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		HardwareDeps: hwdep.D(pre.InputsStableModels),
		VarDeps: []string{
			"ui.gaiaPoolDefault",
			"ui.signinProfileTestExtensionManifestKey",
		},
		Timeout: 3 * time.Minute,
		Params: []testing.Param{
			{
				Name: "tablet",
				Val:  true, // Tablet VK.
			},
			{
				Name: "clamshell",
				Val:  false, // A11y VK.
			},
		},
	})
}

func VirtualKeyboardLoginScreen(ctx context.Context, s *testing.State) {
	// Give 5 seconds to clean up and dump out UI tree.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// Use GAIA login otherwise user profile does not exist after restart UI.
	cr, err := chrome.New(ctx, chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")))
	if err != nil {
		s.Fatal("Failed to start Chrome via GAIA login: ", err)
	}

	// Restart device and keep state to land login page.
	chromeOpts := []chrome.Option{
		chrome.NoLogin(),
		chrome.KeepState(),
		chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")),
	}

	isTabletVK := s.Param().(bool)
	if isTabletVK {
		chromeOpts = append(chromeOpts, chrome.ExtraArgs("--force-tablet-mode=touch_view"), chrome.VKEnabled())
	} else {
		chromeOpts = append(chromeOpts, chrome.ExtraArgs("--force-tablet-mode=clamshell"))
	}
	cr, err = chrome.New(ctx, chromeOpts...)
	if err != nil {
		s.Fatal("Failed to start Chrome after restart: ", err)
	}

	tconn, err := cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	defer faillog.SaveScreenshotOnError(cleanupCtx, cr, s.OutDir(), s.HasError)

	uc, err := inputactions.NewInputsUserContext(ctx, s, cr, tconn, nil)
	if err != nil {
		s.Fatal("Failed to create user context: ", err)
	}

	ui := uiauto.New(tconn)
	ud := uidetection.NewDefault(tconn).WithTimeout(3 * time.Second).WithScreenshotStrategy(uidetection.ImmediateScreenshot)
	vkbCtx := vkb.NewContext(cr, tconn)
	leftShiftKey := nodewith.Name("shift").Ancestor(vkb.NodeFinder.HasClass("key_pos_shift_left"))

	// Manually enable A11y VK and click password field to trigger VK.
	if !isTabletVK {
		if err := uiauto.Combine("trigger A11y VK",
			vkbCtx.EnableA11yVirtualKeyboard(true),
			vkbCtx.ClickUntilVKShown(nodewith.NameContaining("Password").Role(role.TextField)),
		)(ctx); err != nil {
			s.Fatal("Failed to enable A11y VK: ", err)
		}
	}

	// Type password "q2Aa m" to cover letters, capitals, numbers and space.
	passwordText := uidetection.TextBlock([]string{"q2Aa", "m"})

	if err := uiauto.UserAction(
		"VK typing input",
		uiauto.Combine(`input and verify login password`,
			vkbCtx.TapKeys([]string{"q", "2"}),                      // pwd: q2
			vkbCtx.TapNode(leftShiftKey),                            // Shifted VK
			vkbCtx.TapKey("A"),                                      // pwd: q2A
			vkbCtx.TapKeysIgnoringCase([]string{"a", "space", "m"}), // pwd: q2Aa m
			ui.DoDefault(nodewith.Name("Show password")),
			ud.WaitUntilExists(passwordText),
		),
		uc,
		&useractions.UserActionCfg{
			Attributes: map[string]string{
				useractions.AttributeFeature:      useractions.FeatureVKTyping,
				useractions.AttributeInputField:   "Password field on login page",
				useractions.AttributeTestScenario: "Use VK in password field on login page",
			},
		},
	)(ctx); err != nil {
		s.Fatal("Failed to verify VK input in password field on login page: ", err)
	}
}
