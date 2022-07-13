// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/inputs/inputactions"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/bundles/cros/inputs/util"
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
				Name:      "kiosk",
				Fixture:   fixture.KioskAutoLaunchCleanup,
				ExtraAttr: []string{"informational", "group:input-tools-upstream"},
				Val:       ime.EnglishUS,
			},
		},
	})
}

func KioskPhysicalKeyboardCapsLock(ctx context.Context, s *testing.State) {

	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()
	kiosk, cr, err := kioskmode.New(
		ctx,
		fdms,
		kioskmode.DefaultLocalAccounts(),
		kioskmode.AutoLaunch(kioskmode.WebKioskAccountID),
		kioskmode.PublicAccountPolicies(kioskmode.WebKioskAccountID, []policy.Policy{
			// Add policies here.
		}),
	)
	if err != nil {
		s.Fatal("Failed to start Chrome in Kiosk mode: ", err)
	}
	defer kiosk.Close(ctx)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 15*time.Second)
	defer cancel()

	// required since kioskfixture doesn't automatically provide these
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get Test API connection: ", err)
	}
	uc, err := inputactions.NewInputsUserContextWithoutState(ctx, "", s.OutDir(), cr, tconn, nil)
	if err != nil {
		s.Fatal("Failed to create new inputs user context: ", err)
	}

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree")

	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer keyboard.Close()

	// LaunchBrowser doesn't work because browser process cannot be found in kiosk mode.
	its, err := testserver.Launch(ctx, cr, tconn)

	// CloseAll seems to crash due to its.closeBrowser(ctx) failing due to NPE. Close() doens't closeBrowser.
	defer its.Close()

	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	inputField := testserver.TextAreaInputField

	// Pressing the shift key by itself does not have any affect if no caps lock.
	// However if the test failed while caps lock is still enabled, shift will
	// disable it. This is just to make sure the state of the caps lock is clean
	// for the next test that uses the fixture.
	defer keyboard.AccelAction("Shift")(cleanupCtx)

	actionName := "PK enable caps lock, type and disable caps lock"
	if err := uiauto.UserAction(actionName,

		uiauto.Combine(actionName,
			keyboard.AccelAction("Alt+Search"),
			its.Clear(inputField),
			its.ClickFieldAndWaitForActive(inputField),
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
