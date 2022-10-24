// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package privacyhubutil contains utility functions for privacy hub tests.
package privacyhubutil

import (
	"context"
	"fmt"
	"image"
	"image/png"
	"os"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/screenshot"
)

func getImage(ctx context.Context, cr *chrome.Chrome, rct coords.Rect) (image.Image, error) {
	var l int = rct.Left + (rct.Width / 3)
	var t int = rct.Top + (rct.Height / 3)
	var w int = rct.Width / 3
	var h int = rct.Height / 3
	var subR coords.Rect = coords.Rect{Left: l, Top: t, Width: w, Height: h}
	sshot, err := screenshot.GrabAndCropScreenshot(ctx, cr, subR)
	if err != nil {
		return nil, errors.Wrap(err, "failed to grab screenshot")
	}
	return sshot, nil
}

// SaveImage saves a given image.Image as png
func SaveImage(outDir, name string, img image.Image) error {
	f, err := os.Create(fmt.Sprintf("%s/%s", outDir))
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

// IsImageBlack checks whether the given image.Image is black.
func IsImageBlack(img image.Image) (bool, error) {
	if img == nil {
		return false, errors.New("Image is nil")
	}

	var bounds image.Rectangle = img.Bounds()

	for x := bounds.Min.X; x <= bounds.Max.X; x++ {
		for y := bounds.Min.Y; y <= bounds.Max.Y; y++ {
			r, g, b, _ := img.At(x, y).RGBA()
			if r+g+b != 0 {
				return false, nil
			}
		}
	}

	return true, nil
}

// CameraScreenshot takes a screenshot from the Camera app and crops it to contain no UI elements (works in VM resolution)
func CameraScreenshot(ctx context.Context, cr *chrome.Chrome, tconn *browser.TestConn) (image.Image, error) {
	ui := uiauto.New(tconn)
	homeButton := nodewith.Role("button").Name("Launcher").ClassName("ash/HomeButton")
	cameraAppButton := nodewith.Role("button").Name("Camera").ClassName("AppListItemView").First()

	if err := ui.LeftClick(homeButton)(ctx); err != nil {
		errors.Wrap(err, "failed to left click")
	}
	if err := ui.LeftClick(cameraAppButton)(ctx); err != nil {
		errors.Wrap(err, "failed to right click the Camera App")
	}

	cameraBrowserFrame := nodewith.Role("window").ClassName("BrowserFrame").Name("Camera")
	cameraFrame := nodewith.ClassName("RenderWidgetHostViewAura").FinalAncestor(cameraBrowserFrame)
	if err := ui.WithTimeout(10 * time.Second).WaitUntilExists(cameraBrowserFrame)(ctx); err != nil {
		return nil, err
	}

	frameLoc, err := ui.Location(ctx, cameraFrame)
	if err != nil {
		return nil, errors.Wrap(err, "failed to obtain Location")
	}

	var sshot image.Image
	sshot, err = getImage(ctx, cr, *frameLoc)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create camera feed image")
	}
	return sshot, nil

}
