// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"strings"
	"time"

	"github.com/mafredri/cdp/protocol/target"

	"go.chromium.org/chromiumos/tast-tests/local/bundles/cros/inputs/inputactions"
	"go.chromium.org/chromiumos/tast-tests/local/bundles/cros/inputs/pre"
	"go.chromium.org/chromiumos/tast-tests/local/bundles/cros/inputs/util"
	"go.chromium.org/chromiumos/tast-tests/local/chrome"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/uiauto"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/uiauto/faillog"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/uiauto/nodewith"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/uiauto/vkb"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/useractions"
	"go.chromium.org/chromiumos/tast/testing"
	"go.chromium.org/chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardOOBE,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that the virtual keyboard works in OOBE Gaia Login",
		Attr:         []string{"group:mainline", "group:input-tools", "group:input-tools-upstream"},
		Contacts:     []string{"essential-inputs-team@google.com"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		VarDeps:      []string{"inputs.signinProfileTestExtensionManifestKey"},
		HardwareDeps: hwdep.D(pre.InputsStableModels),
	})
}

func VirtualKeyboardOOBE(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.NoLogin(), chrome.VKEnabled(), chrome.ExtraArgs("--force-tablet-mode=touch_view"), chrome.LoadSigninProfileExtension(s.RequiredVar("inputs.signinProfileTestExtensionManifestKey")))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	uc, err := inputactions.NewInputsUserContext(ctx, s, cr, tconn, nil)
	if err != nil {
		s.Fatal("Failed to initiate inputs user context: ", err)
	}

	oobeConn, err := cr.WaitForOOBEConnection(ctx)
	if err != nil {
		s.Fatal("Failed to connect OOBE: ", err)
	}
	defer oobeConn.Close()

	// User lands on GAIA login page afterwards.
	if err := oobeConn.Eval(ctx, "OobeAPI.skipToLoginForTesting()", nil); err != nil {
		s.Fatal("Failed to skip to login: ", err)
	}

	isGAIAWebView := func(t *target.Info) bool {
		return t.Type == "webview" && strings.HasPrefix(t.URL, "https://accounts.google.com/")
	}

	var gaiaConn *chrome.Conn
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var err error
		gaiaConn, err = cr.NewConnForTarget(ctx, isGAIAWebView)
		return err
	}, &testing.PollOptions{Interval: 10 * time.Millisecond}); err != nil {
		s.Fatal("Failed to find GAIA web view: ", err)
	}
	defer gaiaConn.Close()

	const testEmail = "test@gmail.com"

	vkbCtx := vkb.NewContext(cr, tconn)

	userInputFinder := nodewith.Name("Email or phone")
	validateAction := uiauto.Combine("validate virtual keyboard input on OOBE",
		vkbCtx.ClickUntilVKShown(userInputFinder),
		vkbCtx.TapKeys(strings.Split(testEmail, "")),
		// Validate output after tapkeys
		util.WaitForFieldTextToBe(tconn, userInputFinder, testEmail),
	)

	if err := uiauto.UserAction("VK typing input",
		validateAction,
		uc,
		&useractions.UserActionCfg{
			Attributes: map[string]string{
				useractions.AttributeTestScenario: "Use VK in OOBE stage",
				useractions.AttributeInputField:   "OOBE field",
				useractions.AttributeFeature:      useractions.FeatureVKTyping,
			},
		},
	)(ctx); err != nil {
		s.Fatal("Failed to input on OOBE page: ", err)
	}
}
