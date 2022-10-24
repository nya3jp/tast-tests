// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package privacyhub contains tests for privacy hub.
package privacyhub

import (
	"context"
	"image"
	"strconv"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/privacyhub/privacyhubutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/state"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CameraSwitch,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that PrivacyHub camera toggle switches off the camera. Should be used only on VMs where the vivid daemon ensures that there is some colorful pattern present in the camera feed. Also this tests assumes default vm resolution when cropping the image, and hence different resolution might lead to spurious results",
		Contacts:     []string{"janlanik@google.com", "privacy-hub@google.com"},
		SoftwareDeps: []string{"chrome", "qemu"},
		Timeout:      5 * time.Minute,
		Attr:         []string{"group:mainline", "informational"},
	})
}

func hasState(toggleInfo *uiauto.NodeInfo, state state.State) bool {
	if val, present := toggleInfo.State[state]; present {
		return val
	}
	return false
}

func isCameraEnabled(ctx context.Context, tconn *browser.TestConn) (bool, error) {
	settings, err := ossettings.Launch(ctx, tconn)
	if err != nil {
		return false, err
	}
	defer settings.Close(ctx)

	var ui *uiauto.Context = uiauto.New(tconn)
	privacyMenu := nodewith.Name("Privacy controls")
	if err := ui.WithTimeout(10 * time.Second).WaitUntilExists(privacyMenu)(ctx); err != nil {
		return false, err
	}
	cameraLabel := nodewith.Name("Camera").Role(role.ToggleButton)
	// Waiting for the camera toggle to appear
	if err := uiauto.Combine("Access camera toggle in Privacy Hub",
		ui.DoDefault(privacyMenu),
		ui.WaitUntilExists(cameraLabel),
	)(ctx); err != nil {
		return false, errors.Wrap(err, "couldn't access camera toggle in Privacy Hub")
	}

	toggleInfo, err := ui.Info(ctx, cameraLabel)
	if err != nil {
		return false, errors.Wrap(err, "couldn't access camera toggle state")
	}

	var pressedAttribute string = "aria-pressed"
	val, ok := toggleInfo.HTMLAttributes[pressedAttribute]
	if !ok {
		return false, errors.Errorf("HTML attribute %q missing", pressedAttribute)
	}

	boolVal, err := strconv.ParseBool(val)
	if err != nil {
		return false, errors.Errorf("Illegal boolean value for attribute %v: %v", pressedAttribute, val)
	}

	return boolVal, nil
}

func clickCameraToggle(ctx context.Context, tconn *browser.TestConn) error {
	settings, err := ossettings.Launch(ctx, tconn)
	if err != nil {
		return err
	}
	defer settings.Close(ctx)

	var ui *uiauto.Context = uiauto.New(tconn)
	privacyMenu := nodewith.Name("Privacy controls")
	if err := ui.WithTimeout(10 * time.Second).WaitUntilExists(privacyMenu)(ctx); err != nil {
		return err
	}
	// Check that the Privacy Hub section contains the required buttons.
	cameraLabel := nodewith.Name("Camera").Role(role.ToggleButton)
	// Waiting for the camera toggle to appear
	if err := uiauto.Combine("Access camera toggle in Privacy Hub",
		ui.DoDefault(privacyMenu),
		ui.WaitUntilExists(cameraLabel),
	)(ctx); err != nil {
		return errors.Wrap(err, "couldn't access camera toggle in Privacy Hub")
	}

	// Toggling the camera toggle
	if err := ui.DoDefault(cameraLabel)(ctx); err != nil {
		return errors.Wrap(err, "couldn't click at the camera toggle")
	}

	return nil
}

func setCameraSwitchState(ctx context.Context, tconn *browser.TestConn, enabled bool) error {
	currentState, err := isCameraEnabled(ctx, tconn)
	if err != nil {
		return err
	}
	if currentState == enabled {
		return nil
	}
	if err := clickCameraToggle(ctx, tconn); err != nil {
		return err
	}
	return nil
}

func CameraSwitch(ctx context.Context, s *testing.State) {
	// Shorten deadline to leave time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cr, err := chrome.New(ctx, chrome.EnableFeatures("CrosPrivacyHub"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	settings, err := ossettings.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch OS settings: ", err)
	}
	defer settings.Close(cleanupCtx)
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree")

	s.Log("Enabling the camera in Privacy Hub")
	if err := setCameraSwitchState(ctx, tconn, true); err != nil {
		s.Fatal("Failed to switch the camera toggle in the Privacy Hub: ", err)
	}
	enabled, err := isCameraEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to read the camera toggle state: ", err)
	}
	if !enabled {
		s.Fatal("Failed to enable the camera")
	}
	var sshot image.Image
	sshot, err = privacyhubutil.CameraScreenshot(ctx, cr, tconn)
	if err != nil {
		s.Fatal("Couldn't get camera screenshot: ", err)
	}
	isBlack, err := privacyhubutil.IsImageBlack(sshot)
	if err != nil {
		s.Fatal("Failed to verify a camera screenshot: ", err)
	}
	if isBlack {
		privacyhubutil.SaveImage(s.OutDir(), "should_be_colourfull.png", sshot)
		s.Fatal("Camera feed is black even though the camera should be enabled")
	}

	s.Log("Disabling the camera in Privacy Hub")
	if err := clickCameraToggle(ctx, tconn); err != nil {
		s.Fatal("Failed to switch the camera toggle in the Privacy Hub: ", err)
	}
	enabled, err = isCameraEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to read the camera toggle state: ", err)
	}
	if enabled {
		s.Fatal("Failed to disable the camera")
	}
	sshot, err = privacyhubutil.CameraScreenshot(ctx, cr, tconn)
	if err != nil {
		s.Fatal("Couldn't get camera screenshot: ", err)
	}
	isBlack, err = privacyhubutil.IsImageBlack(sshot)
	if err != nil {
		s.Fatal("Failed to verify a camera screenshot: ", err)
	}
	if !isBlack {
		privacyhubutil.SaveImage(s.OutDir(), "should_be_black.png", sshot)
		s.Fatal("Camera feed is not black even though the camera should be disabled")
	}
}
