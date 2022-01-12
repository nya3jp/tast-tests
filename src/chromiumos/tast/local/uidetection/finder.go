// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package uidetection

import (
	"context"
	"math"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/coords"
	pb "chromiumos/tast/local/uidetection/api"
	"chromiumos/tast/testing"
)

const (
	// ErrNotFound is the error when there is no matching elements found.
	ErrNotFound = "no matching elements found"
	// ErrMultipleMatch is the error when there are multiple matching elements found.
	ErrMultipleMatch = "there are multiple matches"
	// ErrNthNotFound is the error when the Nth element doesn't exist.
	ErrNthNotFound = "Nth element not found"
)

// Location represents the location of a matching UI element.
type Location struct {
	// Rectangle of the location.
	coords.Rect
	// Text associated with the element, if any.
	Text string
}

// Finder represents a data structure that consists of arguments to find
// a UI element.
type Finder struct {
	screenshot []byte
	// The request used to construct the finder.
	request *pb.DetectionRequest
	// boundingBoxes stores the locations of the responses from the request.
	boundingBoxes []*Location
	// Descriptor for the finder.
	desc       string
	nth        int
	exactMatch bool
}

func newFinder() *Finder {
	return &Finder{
		nth:        -1,
		exactMatch: false,
	}
}

func newFromRequest(r *pb.DetectionRequest, d string) *Finder {
	return &Finder{
		request:       r,
		boundingBoxes: nil,
		desc:          d,
		nth:           -1,
		exactMatch:    false,
	}
}

// copy returns a copy of the input Finder.
// It copies all of the keys/values in attributes and state individually.
func (s *Finder) copy() *Finder {
	c := newFinder()
	c.screenshot = s.screenshot
	c.request = s.request
	c.boundingBoxes = s.boundingBoxes
	c.desc = s.desc
	c.nth = s.nth
	return c
}

// First enables the finder to choose the first match of a UI element
// if there are multiple matches.
func (s *Finder) First() *Finder {
	c := s.copy()
	c.nth = 0
	return c
}

// Nth enables the finder to choose the n-th match of a UI element.
// if there are multiple matches.
func (s *Finder) Nth(nth int) *Finder {
	c := s.copy()
	c.nth = nth
	return c
}

// ExactMatch turns off the approximate match for the word finder.
// The results will be filtered by exact string matching.
// USE WITH CAUTION. Due to the performance of the OCR (optical character
// recognition) model, approximate match is the default behavior for
// error-tolerance.
// An example use case is when the matching word is short with two or three
// letters.
// TODO(b/211937254): Allow exact matches with max_edit_distance in new proto.
func (s *Finder) ExactMatch() *Finder {
	c := s.copy()
	c.exactMatch = true
	return c
}

// resolve resolves the UI detection request and stores the bounding boxes
// of the matching elements.
func (s *Finder) resolve(ctx context.Context, d *uiDetector, tconn *chrome.TestConn, pollOpts testing.PollOptions, strategy ScreenshotStrategy) error {
	var err error

	switch strategy {
	case StableScreenshot:
		s.screenshot, err = TakeStableScreenshot(ctx, tconn, pollOpts)
		if err != nil {
			return errors.Wrap(err, "failed to take stable screenshot")
		}
	case ImmediateScreenshot:
		s.screenshot, err = TakeScreenshot(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to take screenshot")
		}
	default:
		return errors.New("invalid screenshot strategy")
	}

	screens, err := display.GetInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get the display info")
	}

	// Find the ratio to convert coordinates in the screenshot to those in the screen.
	scaleFactor, err := screens[0].GetEffectiveDeviceScaleFactor()
	if err != nil {
		return errors.Wrap(err, "failed to get the device scale factor")
	}

	// Make sure the scale factor is neither 0 nor NaN.
	if math.IsNaN(scaleFactor) || math.Abs(scaleFactor) < 1e-10 {
		return errors.Errorf("invalid device scale factor: %f", scaleFactor)
	}

	response, err := d.sendDetectionRequest(ctx, s.screenshot, s.request)
	if err != nil {
		return errors.Wrap(err, "failed to resolve the UI detection request")
	}

	s.boundingBoxes = []*Location{}
	for _, boundingBox := range response.BoundingBoxes {
		if s.exactMatch && !strings.EqualFold(boundingBox.GetText(), s.desc) {
			continue
		}
		s.boundingBoxes = append(
			s.boundingBoxes,
			&Location{
				Rect: coords.NewRectLTRB(
					int(float64(boundingBox.GetLeft())/scaleFactor),
					int(float64(boundingBox.GetTop())/scaleFactor),
					int(float64(boundingBox.GetRight())/scaleFactor),
					int(float64(boundingBox.GetBottom())/scaleFactor)),
				Text: boundingBox.GetText(),
			})
	}
	return nil
}

func (s *Finder) location() (*Location, error) {
	if s.boundingBoxes == nil {
		return nil, errors.New("the finder is not resolved")
	}
	numMatches := len(s.boundingBoxes)
	switch {
	case numMatches == 0:
		return nil, errors.New(ErrNotFound)
	case numMatches == 1:
		if s.nth > 0 {
			return nil, errors.Errorf("%s: find only one element, but want the %d-th one", ErrNthNotFound, s.nth)
		}
		return s.boundingBoxes[0], nil
	default: // case numMatches > 1.
		if s.nth < 0 {
			return nil, errors.Errorf("%s: found %d elements. If it is expected, consider using First() or Nth()", ErrMultipleMatch, numMatches)
		}
		if s.nth > numMatches-1 {
			return nil, errors.Errorf("%s: find %d elements, but want the %d-th one", ErrNthNotFound, numMatches, s.nth)
		}
		return s.boundingBoxes[s.nth], nil
	}
}
