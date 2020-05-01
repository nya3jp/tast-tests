// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package appposition is a helper module for the AppPositionTest (chromiumos/tast/local/bundles/cros/arc/app_position.go)
package appposition

import (
	"fmt"
	"image"
	"image/color"

	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

func min(a, b uint32) uint32 {
	if a < b {
		return a
	}
	return b
}

func max(a, b uint32) uint32 {
	if a < b {
		return b
	}
	return a
}

// colorErrorTolerance is the tolerance of color difference between expected color and the color to verify.
const colorErrorTolerance = 6

func getBackgroundColor() color.Color {
	return color.RGBA{0, 0, 255, 255} // Blue
}
func getShadowedBackgroundColor() color.Color {
	return color.RGBA{0, 0, 254 - colorErrorTolerance, 255} // Grey blue
}
func getContentColor() color.Color {
	return color.RGBA{0, 255, 0, 255} // Green
}

type colorPredicate interface {
	call(c color.Color) bool
	getExpectedColor() string
}

func verifyRegion(s *testing.State, img image.Image, rect coords.Rect, predicate colorPredicate) {
	// verifyRegion verifies that all pixels in rect in img passes the test of
	// predicate.

	if !(img.Bounds().Min.X <= rect.Left && rect.Left+rect.Width <= img.Bounds().Max.X && img.Bounds().Min.Y <= rect.Top && rect.Top+rect.Height <= img.Bounds().Max.Y) {
		s.Errorf("Region %s exceeds image bounds %s", rect, img.Bounds())
	}

	for y := rect.Top; y < rect.Top+rect.Height; y++ {
		for x := rect.Left; x < rect.Left+rect.Width; x++ {
			c := img.At(x, y)
			if !predicate.call(c) {
				s.Errorf("Pixel color at (%d, %d) is not valid: %s. Expected: %s", x, y, c, predicate.getExpectedColor())
				return
			}
		}
	}
}

// shadowedColorPredicate is the precidate that verifies if a color is the expected color.
//
// This predicate accepts expected color with tolerance colorErrorTolerance.
// However, the tolerance is ignored for components whose values are zero. For example, if pure black is given, it only matches pure black.
type shadowedColorPredicate struct {
	expectedColor color.Color
}

func (predicate shadowedColorPredicate) call(c color.Color) bool {
	r1, g1, b1, _ := c.RGBA()
	r2, g2, b2, _ := predicate.RGBA()
	if r2 > 0 && !(0 < r1 && r1 <= int64(r2)+colorErrorTolerance*0x0100) {
		return false
	}
	if g2 > 0 && !(0 < g1 && g1 <= int64(g2)+colorErrorTolerance*0x0100) {
		return false
	}
	if b2 > 0 && !(0 < b1 && b1 <= int64(b2)+colorErrorTolerance*0x0100) {
		return false
	}
	return true
}

func (predicate shadowedColorPredicate) getExpectedColor() string {
	return fmt.Sprintf("%s", predicate.expectedColor)
}

// VerifyContent checks whether the given area of the screenshot is the content of the app.
func VerifyContent(s *testing.State, img image.Image, contentBounds coords.Rect) {
	s.Logf("Verifying content color in %s", contentBounds)
	verifyRegion(s, img, contentBounds, ShadowedColorPredicate{getContentColor()})
}

// VerifyShadow checks whether the given areas of the screenshot is the shadow of the window of the app.
func VerifyShadow(s *testing.State, img image.Image, shadowBoundsList []coords.Rect) {
	for _, shadowBounds := range shadowBoundsList {
		s.Logf("Verifying shadow color in %s", shadowBounds)
		verifyRegion(s, img, shadowBounds, ShadowedColorPredicate{getShadowedBackgroundColor()})
	}
}

const captionColorComponentThreshold = 30

// captionColorPredicate is the predicate that verifies that a color can be used in a caption as
// background.
//
// Chrome side caption and Android side caption have different spec, but they
// share some common traits:
// 1) They have positive values in all RGB color components;
// 2) They're grey-ish (max of RGB component is smaller than min of RGB
//    component + 30).
//
// These traits should be enough for us to tell the difference from common
// graphical flaws in our test settings.
//
// TODO(b/79587124): Find a better predicate for Chrome side caption color.
type captionColorPredicate struct {
}

func (predicate captionColorPredicate) call(c color.Color) bool {
	r, g, b, _ := c.RGBA()
	return r > 0 && g > 0 && b > 0 && max(r, max(g, b))-min(r, min(g, b)) < captionColorComponentThreshold
}

func (predicate captionColorPredicate) getExpectedColor() string {
	return "(r > 0, g > 0, b > 0) && r ~= g ~= b"
}

// VerifyCaption checks whether the given area of the given screenshot is the caption of the window of the app.
func VerifyCaption(s *testing.State, img image.Image, captionBounds coords.Rect) {
	s.Logf("Verifying caption color in %s", captionBounds)
	verifyRegion(s, img, captionBounds, captionColorPredicate{})
}
