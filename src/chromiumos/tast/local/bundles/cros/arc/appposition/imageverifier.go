// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package appposition

import (
	"image"
	"fmt"
	"image/color"
	"chromiumos/tast/testing"
	"chromiumos/tast/local/coords"
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



// Tolerance of color difference between expected color and the color to verify.
const colorErrorTolerance = 6

// Content colors
func getBackgroundColor() color.Color {
	return color.RGBA{0, 0, 255, 255}  // Blue
}
func getShadowedBackgroundColor() color.Color {
	return color.RGBA{0, 0, 254 - colorErrorTolerance, 255}  // Grey blue
}
func getContentColor() color.Color {
	return color.RGBA{0, 255, 0, 255}  // Green
}

type ColorPredicate interface {
    call(c color.Color) bool
    getExpectedColor() string
}

func verifyRegion(s *testing.State, img image.Image, rect coords.Rect, predicate ColorPredicate) {
    // Verifies that all pixels in |rect| in |image| passes the test of
    // verifier.

    // The region includes the left and top boundary of |rect|, and excludes the
    // right and bottom boundary.

    // @param image: the image to verify. Should be a PIL Image object.
    // @param rect: the region to verify. Should be Rect object.
    // @param predicate: the predicate to verify a pixel's color. Should be a
    //         function or a functor that takes a 3-tuple denoting the color and
    //         return true if color is valid.

    // @raise AssertException if region exceeds image size.
    // @raise test.TestError if pixel is not valid.

    if ! (img.Bounds().Min.X <= rect.Left && rect.Left + rect.Width <= img.Bounds().Max.X && img.Bounds().Min.Y <= rect.Top && rect.Top + rect.Height <= img.Bounds().Max.Y) {
	    s.Errorf("Region %s exceeds image bounds %s", rect, img.Bounds())
    }

    for y := rect.Top; y < rect.Top + rect.Height; y++ {
	    for x := rect.Left; x < rect.Left + rect.Width; x++ {
		    c := img.At(x, y)
		    if ! predicate.call(c) {
			    s.Errorf("Pixel color at (%d, %d) is not valid: %s. Expected: %s", x, y, c, predicate.getExpectedColor())
			    return
		    }
	    }
    }
}

// A precidate that verifies if a color is the expected color, or a ratio
// multiplied by the expected color.
//
// For performance consideration, this predicate only accepts expected color
// that has at most one color component (Red, Green, or Blue) with tolerance
// |colorErrorTolerance|. If pure black is given, it only matches pure
// black.
type ShadowedColorPredicate struct {
	expectedColor color.Color
	component string
	value uint32
}

func NewShadowedColorPredicate(expectedColor color.Color) (ShadowedColorPredicate, error) {
	r, g, b, _ := expectedColor.RGBA()
	component := "?"
	value := uint32(0)
	if r > 0 {
		component = "R"
		value = r
	}
	if g > 0 {
		if component != "?" {
			return ShadowedColorPredicate{}, fmt.Errorf("%#v has more then 1 components.", expectedColor)
		}
		component = "G"
		value = g
	}
	if b > 0 {
		if component != "?" {
			return ShadowedColorPredicate{}, fmt.Errorf("%#v has more then 1 components.", expectedColor)
		}
		component = "B"
		value = b
	}

	return ShadowedColorPredicate {
		expectedColor,
		component,
		min(value + colorErrorTolerance * 0x0100, 0xffff),
	}, nil
}

func (predicate ShadowedColorPredicate) call(c color.Color) bool {
	r, g, b, _ := c.RGBA()
	numOfComponents := 0
	if r > 0 {
		numOfComponents += 1
	}
	if g > 0 {
		numOfComponents += 1
	}
	if b > 0 {
		numOfComponents += 1
	}
	if numOfComponents > 1 {
		// This color has more than one component
		return false
	}

	value := uint32(0)
	if predicate.component == "R" {
		value = r
	} else if predicate.component == "G" {
		value = g
	} else if predicate.component == "B" {
		value = b
	}
        return (value > 0 && value <= predicate.value) || numOfComponents == 0
}

func (predicate ShadowedColorPredicate) getExpectedColor() string {
        return fmt.Sprintf("%s", predicate.expectedColor)
}

func VerifyContent(s *testing.State, img image.Image, contentBounds coords.Rect) {
	s.Logf("Verifying content color in %s.", contentBounds)
	predicate, err := NewShadowedColorPredicate(getContentColor())
	if err != nil {
		s.Errorf("Failed to make shadowed color predicate: ", err)
	} else {
		verifyRegion(s, img, contentBounds, predicate)
	}
}


func VerifyShadow(s *testing.State, img image.Image, shadowBoundsList []coords.Rect) {
	for _, shadowBounds := range shadowBoundsList {
		s.Logf("Verifying shadow color in %s.", shadowBounds)
		predicate, err := NewShadowedColorPredicate(getShadowedBackgroundColor())
		if err != nil {
			s.Errorf("Failed to make shadowed color predicate: ", err)
		} else {
			verifyRegion(s, img, shadowBounds, predicate)
		}
	}
}


// Caption color threshold
const captionColorComponentThreshold = 30

// A predicate that verifies that a color can be used in a caption as
// background.
//
// Chrome side caption and Android side caption have different spec, but they
// share some common traits:
// 1) They have positive values in all RGB color components;
// 2) They're grey-ish (max of RGB component is smaller than min of RGB
//    component + 30).
//
// TODO(b/79587124): Find a better predicate for Chrome side caption color.
type CaptionColorPredicate struct {
}

// These traits should be enough for us to tell the difference from common
// graphical flaws in our test settings.
func (predicate CaptionColorPredicate) call(c color.Color) bool {
	r, g, b, _ := c.RGBA()
	return r > 0 && g > 0 && b > 0 && max(r, max(g, b)) - min(r, min(g, b)) < captionColorComponentThreshold
}

func (predicate CaptionColorPredicate) getExpectedColor() string {
        return "(r > 0, g > 0, b > 0) && r ~= g ~= b"
}

func VerifyCaption(s *testing.State, img image.Image, captionBounds coords.Rect) {
    s.Logf("Verifying caption color in %s.", captionBounds)
    verifyRegion(s, img, captionBounds, CaptionColorPredicate{})
}
