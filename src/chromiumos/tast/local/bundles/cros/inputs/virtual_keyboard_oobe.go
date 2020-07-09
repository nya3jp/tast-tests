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

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/vkb"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardOOBE,
		Desc:         "Checks that the virtual keyboard works in OOBE Gaia Login",
		Attr:         []string{"group:mainline", "informational"},
		Contacts:     []string{"essential-inputs-team@google.com"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		HardwareDeps: hwdep.D(hwdep.SkipOnModel(pre.ExcludeModels...)),
	})
}

func VirtualKeyboardOOBE(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.ExtraArgs("--enable-virtual-keyboard"), chrome.ExtraArgs("--force-tablet-mode=touch_view"), chrome.NoLogin())
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	oobeConn, err := cr.WaitForOOBEConnection(ctx)
	if err != nil {
		s.Fatal("Failed to connect OOBE: ", err)
	}

	// User lands on GAIA login page afterwards.
	if err := oobeConn.Exec(ctx, "Oobe.skipToLoginForTesting()"); err != nil {
		s.Fatal("Failed to skip to login: ", err)
	}

	isGAIAWebView := func(t *target.Info) bool {
		return t.Type == "webview" && strings.HasPrefix(t.URL, "https://accounts.google.com/")
	}

	gaiaConn, err := cr.NewConnForTarget(ctx, isGAIAWebView)
	if err != nil {
		s.Fatal("Failed to connect to GAIA webview: ", err)
	}
	defer gaiaConn.Close()

	kconn, err := vkb.UIConn(ctx, cr)
	if err != nil {
		s.Fatal("Creating connection to virtual keyboard UI failed: ", err)
	}
	defer kconn.Close()

	if err = inputField(ctx, kconn, gaiaConn, "#identifierId", []string{"t", "e", "s", "t", "@", "g", "m", "a", "i", "l", ".", "c", "o", "m"}, "test@gmail.com"); err != nil {
		s.Error("Failed to input identifierId with vk in user login: ", err)
	}
}

func inputField(ctx context.Context, kconn, gaiaConn *chrome.Conn, cssSelector string, keys []string, expectedValue string) error {
	// Wait for document to load and input field to appear.
	if err := gaiaConn.WaitForExpr(ctx, fmt.Sprintf(
		"!!document.activeElement && document.querySelector(%q)===document.activeElement", cssSelector)); err != nil {
		return errors.Wrapf(err, "failed to wait for document ready or %q element", cssSelector)
	}

	// Original view port size without virtual keyboard
	originalViewPortSize, err := getViewPortSize(ctx, gaiaConn)
	if err != nil {
		return errors.Wrap(err, "failed to get viewport size")
	}

	//TODO(b/159748349): Tap input field to trigger virtual keyboard rather than JS function.
	if err := kconn.Eval(ctx, "chrome.inputMethodPrivate.showInputView()", nil); err != nil {
		return errors.Wrap(err, "failed to show virtual keyboard via JS")
	}

	// Wait for viewport shrink vertically because of vk showing up.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		newViewPort, err := getViewPortSize(ctx, gaiaConn)

		if err != nil {
			return errors.Wrap(err, "failed to get viewport size")
		}

		if newViewPort.Height >= originalViewPortSize.Height {
			return errors.Errorf(`original viewport size: %v; latest viewport size: %v`, originalViewPortSize, newViewPort)
		}
		return nil
	}, &testing.PollOptions{Timeout: 3 * time.Second}); err != nil {
		return errors.Wrap(err, "viewport does not shrink in height after touching input field to show virtual keyboard")
	}

	// Tap keys sequentially to input
	vkb.TapKeysJS(ctx, kconn, keys)

	// Wait for the text field to have the correct contents
	if err := gaiaConn.WaitForExpr(ctx, fmt.Sprintf(
		`document.querySelector(%q).value === %q`, cssSelector, expectedValue)); err != nil {
		return errors.Wrap(err, "failed to get the contents of the text field")
	}

	return nil
}

func getViewPortSize(ctx context.Context, conn *chrome.Conn) (coords.Size, error) {
	var vpSize coords.Size
	if err := conn.Eval(ctx, `(function() {
		  return {'height': window.innerHeight, 'width': window.innerWidth};
	  })()`, &vpSize); err != nil {
		return vpSize, errors.Wrap(err, "failed to get viewport size")
	}
	return vpSize, nil
}

func getScreenSize(ctx context.Context, conn *chrome.Conn) (coords.Size, error) {
	var screenSize coords.Size
	if err := conn.Eval(ctx, `(function() {
		  return {'height': screen.height, 'width': screen.width};
	  })()`, &screenSize); err != nil {
		return screenSize, errors.Wrap(err, "failed to get viewport size")
	}
	return screenSize, nil
}
