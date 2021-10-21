// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package uidetection provides image-based UI detections/interactions.
package uidetection

import (
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
	"context"
	"time"
)

// Location is a rectangle containing a matching UI element.
type Location struct {
	TopLeft     coords.Point
	BottomRight coords.Point
}

// UiDetectionAuto provides functionalities for image-based UI automation.
type UiDetectionAuto struct {
	tconn    *chrome.TestConn
	detector *uiDetector
}

// NewAUiDetectionAuto returns a new UiDetectionAuto instance.
func NewAUiDetectionAuto(t *chrome.TestConn, s *testing.State) *UiDetectionAuto {
	return &UiDetectionAuto{
		tconn: t,
		detector: &uiDetector{
			keyType: s.RequiredVar("uidetection.key_type"),
			key:     s.RequiredVar("uidetection.key"),
			server:  s.RequiredVar("uidetection.server"),
		},
	}

}

// LocationOfWord finds the location of a word in the screen.
func (uda *UiDetectionAuto) LocationOfWord(ctx context.Context, word string) (*Location, error) {
	imagePng, err := TakeScreenshot(ctx)
	if err != nil {
		return nil, err
	}

	return Word(word).find(ctx, uda.detector, imagePng)
}

// LocationOfTextBlock finds the location of a text block containing several words in the screen.
func (uda *UiDetectionAuto) LocationOfTextBlock(ctx context.Context, words []string) (*Location, error) {
	imagePng, err := TakeScreenshot(ctx)
	if err != nil {
		return nil, err
	}

	return TextBlock(words).find(ctx, uda.detector, imagePng)
}

// LocationOfCustomIcon finds the location of a custom icon in the screen.
func (uda *UiDetectionAuto) LocationOfCustomIcon(ctx context.Context, iconFile string) (*Location, error) {
	imagePng, err := TakeScreenshot(ctx)
	if err != nil {
		return nil, err
	}
	// Read icon image from file.
	icon, err := ReadImage(iconFile)
	if err != nil {
		return nil, err
	}

	return CustomIcon(icon).find(ctx, uda.detector, imagePng)
}

// ClickWord returns an action that clicks a word.
func (uda *UiDetectionAuto) ClickWord(word string) uiauto.Action {
	return func(ctx context.Context) error {
		return testing.Poll(ctx,
			func(ctx context.Context) error {
				loc, err := uda.LocationOfWord(ctx, word)
				if err != nil {
					return errors.Wrapf(err, "failed to find the location of word %q", word)
				}
				return uda.clickLocation(loc)(ctx)
			},
			&testing.PollOptions{Timeout: 50 * time.Second},
		)
	}
}

// ClickTextBlock returns an action that clicks a textblock.
func (uda *UiDetectionAuto) ClickTextBlock(words []string) uiauto.Action {
	return func(ctx context.Context) error {
		return testing.Poll(ctx,
			func(ctx context.Context) error {
				loc, err := uda.LocationOfTextBlock(ctx, words)
				if err != nil {
					return errors.Wrapf(err, "failed to find the location of textblock %q", words)
				}
				return uda.clickLocation(loc)(ctx)
			},
			&testing.PollOptions{Timeout: 50 * time.Second},
		)
	}
}

// ClickCustomIcon returns an action that clicks a custom icon.
func (uda *UiDetectionAuto) ClickCustomIcon(iconFile string) uiauto.Action {
	return func(ctx context.Context) error {
		return testing.Poll(ctx,
			func(ctx context.Context) error {
				loc, err := uda.LocationOfCustomIcon(ctx, iconFile)
				if err != nil {
					return errors.Wrap(err, "failed to find the location of custom icon")
				}
				return uda.clickLocation(loc)(ctx)
			},
			&testing.PollOptions{Timeout: 50 * time.Second},
		)
	}
}

func (uda *UiDetectionAuto) clickLocation(loc *Location) uiauto.Action {
	center := coords.NewPoint((loc.TopLeft.X+loc.BottomRight.X)/2, (loc.TopLeft.Y+loc.BottomRight.Y)/2)
	return mouse.Click(uda.tconn, center, mouse.LeftButton)
}
