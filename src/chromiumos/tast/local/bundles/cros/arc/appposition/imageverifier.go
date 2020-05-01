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

// to16bit cast 8 bit values (in [0, 0xff]) to 16 bit values (in [0, 0xffff]) for the color.Color's RGBA().
func to16bit(c uint8) uint32 {
	return uint32(c) * 0x0101
}

// verifyRegion verifies that all pixels in rect in img passes the test of predicate
//
// The expectation argument is used only for error messages.
func verifyRegion(s *testing.State, img image.Image, rect coords.Rect, predicate func(color.Color) bool, expectation string) {
	if rect.Left < img.Bounds().Min.X || img.Bounds().Max.X < rect.Left+rect.Width || rect.Top < img.Bounds().Min.Y || img.Bounds().Max.Y < rect.Top+rect.Height {
		s.Errorf("Region %v exceeds image bounds %v", rect, img.Bounds())
	}

	for y := rect.Top; y < rect.Top+rect.Height; y++ {
		for x := rect.Left; x < rect.Left+rect.Width; x++ {
			c := img.At(x, y)
			if !predicate(c) {
				s.Errorf("Pixel color at (%d, %d) is not valid: %#v. Expected: %s", x, y, c, expectation)
				return
			}
		}
	}
}

// shadowedColorPredicate is the precidate that verifies if a color is the expected color.
//
// This predicate accepts colors whose values of components are less than the values of corresponding components of expected color with tolerance.
// The tolerance is ignored for components whose expected values are zero. For example, if pure black is expected, it only matches pure black.
//
// For performance consideration, this predicate only accepts expected color that has at most one color component (Red, Green, or Blue).
func shadowedColorPredicate(expectedColor, c color.Color) bool {
	tolerance := to16bit(colorErrorTolerance)

	r1, g1, b1, _ := c.RGBA()
	numOfComponents := 0
	if r1 > tolerance {
		numOfComponents++
	}
	if g1 > tolerance {
		numOfComponents++
	}
	if b1 > tolerance {
		numOfComponents++
	}
	if numOfComponents == 0 {
		return true
	} else if numOfComponents >= 2 {
		// This color has more than one component
		return false
	}

	r2, g2, b2, _ := expectedColor.RGBA()
	if (r2 > tolerance) && !(0 < r1 && r1 <= r2+tolerance) {
		return false
	}
	if (g2 > tolerance) && !(0 < g1 && g1 <= g2+tolerance) {
		return false
	}
	if (b2 > tolerance) && !(0 < b1 && b1 <= b2+tolerance) {
		return false
	}
	return true
}

func newShadowedColorPredicate(expectedColor color.Color) (func(color.Color) bool, string) {
	predicate := func(c color.Color) bool { return shadowedColorPredicate(expectedColor, c) }
	expectation := fmt.Sprintf("%#v with a shadow", expectedColor)
	return predicate, expectation
}

// VerifyContent checks whether the given area of the screenshot is the content of the app.
func VerifyContent(s *testing.State, img image.Image, contentBounds coords.Rect) {
	predicate, expectation := newShadowedColorPredicate(getContentColor())
	s.Log("Verifying content color in ", contentBounds)
	verifyRegion(s, img, contentBounds, predicate, expectation)
}

// VerifyShadow checks whether the given areas of the screenshot is the shadow of the window of the app.
func VerifyShadow(s *testing.State, img image.Image, shadowBoundsList []coords.Rect) {
	predicate, expectation := newShadowedColorPredicate(getShadowedBackgroundColor())
	for _, shadowBounds := range shadowBoundsList {
		s.Log("Verifying shadow color in ", shadowBounds)
		verifyRegion(s, img, shadowBounds, predicate, expectation)
	}
}

const captionColorComponentThreshold = 30

// captionColorPredicate is the predicate that verifies that a color can be used in a caption as
// background.
//
// Chrome side caption and Android side caption have different spec, but they
// share some common traits:
// 1) They have positive values in all RGB color components;
// 2) They're grey-ish (max of RGB component is smaller than min of RGB component + 30).
//
// These traits should be enough for us to tell the difference from common
// graphical flaws in our test settings.
//
// TODO(b/79587124): Find a better predicate for Chrome side caption color.
func captionColorPredicate(c color.Color) bool {
	threshold := to16bit(captionColorComponentThreshold)
	r, g, b, _ := c.RGBA()
	return r > 0 && g > 0 && b > 0 && max(r, max(g, b))-min(r, min(g, b)) < threshold
}

const captionColorPredicateExpectation = "(r > 0, g > 0, b > 0) && r ~= g ~= b"

// VerifyCaption checks whether the given area of the given screenshot is the caption of the window of the app.
func VerifyCaption(s *testing.State, img image.Image, captionBounds coords.Rect) {
	s.Log("Verifying caption color in ", captionBounds)
	verifyRegion(s, img, captionBounds, captionColorPredicate, captionColorPredicateExpectation)
}
