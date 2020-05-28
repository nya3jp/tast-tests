// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mafredri/cdp/protocol/target"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/vkb"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardOOBE,
		Desc:         "Checks that the virtual keyboard works in OOBE Gaia Login",
		Attr:         []string{"group:mainline", "informational"},
		Contacts:     []string{"essential-inputs-team@google.com"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
	})
}

func VirtualKeyboardOOBE(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.ExtraArgs("--enable-virtual-keyboard"), chrome.NoLogin())
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

	if err = inputField(ctx, kconn, gaiaConn, "input", []string{"t", "e", "s", "t", "@", "g", "m", "a", "i", "l", ".", "c", "o", "m"}, "test@gmail.com"); err != nil {
		s.Fatal("Failed to input identifierId with vk in user login: ", err)
	}
}

func inputField(ctx context.Context, kconn, gaiaConn *chrome.Conn, cssSelector string, keys []string, expectedValue string) error {
	// Wait for document to load and input field to appear.
	if err := gaiaConn.WaitForExpr(ctx, fmt.Sprintf(
		"document.readyState === 'complete' && document.querySelector(%q)", cssSelector)); err != nil {
		return errors.Wrapf(err, "failed to wait for document ready or %q element", cssSelector)
	}

	// Get touch point on input field.
	// Note: for unknown reason, touching Math.round(b.top) + b.height / 2 does not trigger vk.
	var inputFieldTouchPoint coords.Point
	if err := gaiaConn.Eval(ctx, fmt.Sprintf(
		`(function() {
			var elements = document.querySelectorAll(%q);
			for(var element of elements){
				if(!element.hidden){
					var b = element.getBoundingClientRect();
					return {
						'x': Math.round(b.left + b.width / 2),
						'y': Math.round(b.top),
					};
				}
			}
		  })()`, cssSelector), &inputFieldTouchPoint); err != nil {
		return errors.Wrapf(err, "failed to get location of %q element", cssSelector)
	}

	tsw, err := input.Touchscreen(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to open touchscreen device")
	}
	defer tsw.Close()

	// Touchscreen bounds: The size of the touchscreen might not be the same
	// as the display size. In fact, might be even up to 4x bigger.
	// NewTouchCoordConverter is used for convert pixel to touch point.
	fullScreenSize, err := getViewPortSize(ctx, gaiaConn)
	if err != nil {
		return errors.Wrap(err, "failed to get viewport size")
	}

	tcc := tsw.NewTouchCoordConverter(fullScreenSize)
	touchpointX, touchpointY := tcc.ConvertLocation(inputFieldTouchPoint)

	testing.ContextLog(ctx, "touch window size: ", tsw.Width(), tsw.Height())
	testing.ContextLog(ctx, "touch position: ", input.TouchCoord(touchpointX), input.TouchCoord(touchpointY))

	stw, err := tsw.NewSingleTouchWriter()
	if err != nil {
		return errors.Wrap(err, "could not get a new TouchEventWriter")
	}
	defer stw.Close()

	//TODO(shengjun): Investigate why single touch does not trigger virtual keyboard. It requires double touch to show virtual keyboard.
	// In manual testing, single touch can trigger virtual keyboard shown.
	stw.Move(input.TouchCoord(touchpointX), input.TouchCoord(touchpointY))
	stw.End()
	testing.Sleep(ctx, 50*time.Millisecond)
	stw.Move(input.TouchCoord(touchpointX), input.TouchCoord(touchpointY))
	stw.End()

	// Wait for viewport shrink vertically because of vk showing up.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		newViewPort, err := getViewPortSize(ctx, gaiaConn)

		if err != nil {
			return errors.Wrap(err, "failed to get viewport size")
		}

		if newViewPort.Height == fullScreenSize.Height {
			return errors.New("Viewport has not changed yet")
		} else if newViewPort.Height > fullScreenSize.Height {
			// This should not happen in theory
			return errors.New("View port is getting larger during test")
		}
		return nil
	}, &testing.PollOptions{Timeout: 3 * time.Second}); err != nil {
		return errors.Wrap(err, "viewport does not shrink in height after touching input field to show virtual keyboard")
	}

	// Tap keys sequentially to input
	vkb.TapKeys(ctx, kconn, keys)

	// Wait for the text field to have the correct contents
	if err := gaiaConn.WaitForExpr(ctx, fmt.Sprintf(
		`document.querySelector(%q).value === %q`, cssSelector, expectedValue)); err != nil {
		return errors.Wrap(err, "failed to get the contents of the text field")
	}

	// Tap key to hide vk
	vkb.TapKey(ctx, kconn, "hide keyboard")

	// Wait for viewport reverted to full screen because of vk hidden.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		newViewPort, err := getViewPortSize(ctx, gaiaConn)
		if err != nil {
			return errors.Wrap(err, "failed to get viewport size")
		}

		if newViewPort.Height < fullScreenSize.Height {
			return errors.New("Viewport has not reverted to full screen yet")
		} else if newViewPort.Height > fullScreenSize.Height {
			// This should not happen in theory
			return errors.New("View port is getting larger than full screen during test")
		}
		return nil
	}, &testing.PollOptions{Timeout: 3 * time.Second}); err != nil {
		return errors.Wrap(err, "viewport does not reverted back to full screen after hiding virtual keyboard")
	}
	return nil
}

func getViewPortSize(ctx context.Context, conn *chrome.Conn) (coords.Size, error) {
	var vpSize coords.Size
	if err := conn.Eval(ctx, `(function() {
		  return {'height':window.innerHeight, 'width':window.innerWidth};
	  })()`, &vpSize); err != nil {
		return vpSize, errors.Wrap(err, "failed to get viewport size")
	}
	return vpSize, nil
}
