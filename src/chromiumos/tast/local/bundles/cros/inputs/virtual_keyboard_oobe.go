// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mafredri/cdp/protocol/target"

	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/vkb"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardOOBE,
		Desc:         "Checks that the virtual keyboard works in OOBE Gaia Login",
		Attr:         []string{"group:mainline", "group:input-tools"},
		Contacts:     []string{"essential-inputs-team@google.com"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Vars:         []string{"inputs.signinProfileTestExtensionManifestKey"},
		HardwareDeps: pre.InputsStableModels,
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

	oobeConn, err := cr.WaitForOOBEConnection(ctx)
	if err != nil {
		s.Fatal("Failed to connect OOBE: ", err)
	}
	defer oobeConn.Close()

	// User lands on GAIA login page afterwards.
	if err := oobeConn.Eval(ctx, "Oobe.skipToLoginForTesting()", nil); err != nil {
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

	const (
		inputElementCSSLocator = "#identifierId"
		testEmail              = "test@gmail.com"
	)

	element, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{Name: "Email or phone"}, 20*time.Second)
	if err != nil {
		s.Fatal("Failed to find user name input: ", err)
	}
	defer element.Release(ctx)

	s.Log("Click input to trigger virtual keyboard")
	if err := vkb.ClickUntilVKShown(ctx, tconn, element); err != nil {
		s.Fatal("Failed to click the input node and wait for vk shown: ", err)
	}

	if err := gaiaConn.WaitForExpr(ctx, fmt.Sprintf(
		"!!document.activeElement && document.querySelector(%q)===document.activeElement", inputElementCSSLocator)); err != nil {
		s.Fatalf("Failed to wait for document ready or %q element: %v", inputElementCSSLocator, err)
	}

	if err := vkb.TapKeys(ctx, tconn, strings.Split(testEmail, "")); err != nil {
		s.Fatal("Failed to input with virtual keyboard: ", err)
	}

	// Wait for the text field to have the correct contents
	if err := gaiaConn.WaitForExpr(ctx, fmt.Sprintf(
		`document.querySelector(%q).value === %q`, inputElementCSSLocator, testEmail)); err != nil {
		s.Fatal("Failed to validate the contents of the text field: ", err)
	}
}
