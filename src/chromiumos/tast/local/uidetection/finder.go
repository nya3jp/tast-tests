// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package uidetection

import (
	"bytes"
	"context"
	"image/png"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
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
	// The request used to construct the finder.
	request *pb.DetectionRequest
	// boundingBoxes stores the locations of the responses from the request.
	boundingBoxes []*Location
	// Descriptor for the finder.
	desc string
	nth  int
}

func newFinder() *Finder {
	return &Finder{
		nth: -1,
	}
}

func newFromRequest(r *pb.DetectionRequest, d string) *Finder {
	return &Finder{
		request:       r,
		boundingBoxes: nil,
		desc:          d,
		nth:           -1,
	}
}

// copy returns a copy of the input Finder.
// It copies all of the keys/values in attributes and state individually.
func (s *Finder) copy() *Finder {
	c := newFinder()
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

// resolve resolves the UI detection request and stores the bounding boxes
// of the matching elements.
func (s *Finder) resolve(ctx context.Context, d *uiDetector, tconn *chrome.TestConn, pollOpts testing.PollOptions) error {
	// Take the screenshot.
	imagePng, err := TakeStableScreenshot(ctx, pollOpts)
	if err != nil {
		return errors.Wrap(err, "failed to take screenshot")
	}

	// Find the sizes of the screenshot and the screen window to calculate the conversion ratios.
	desktopNode, err := uiauto.New(tconn).Info(ctx, nodewith.Role(role.Window).Name("Built-in display"))
	if err != nil {
		return errors.Wrap(err, "failed to find the desktop node")
	}
	imgBuf := new(bytes.Buffer)
	imgBuf.Write(imagePng)
	img, err := png.Decode(imgBuf)
	if err != nil {
		return errors.Wrap(err, "failed to decode image from bytes")
	}
	// This ratio is used to convert coordinates in the screenshot to coordinates in the screen.
	widthRatio := float64(desktopNode.Location.Width) / float64(img.Bounds().Dx())
	heightRatio := float64(desktopNode.Location.Width) / float64(img.Bounds().Dx())

	response, err := d.sendDetectionRequest(ctx, imagePng, s.request)
	if err != nil {
		return errors.Wrap(err, "failed to resolve the UI detection request")
	}

	s.boundingBoxes = []*Location{}
	for _, boundingBox := range response.BoundingBoxes {
		s.boundingBoxes = append(
			s.boundingBoxes,
			&Location{
				Rect: coords.NewRectLTRB(
					int(float64(boundingBox.GetLeft())*widthRatio),
					int(float64(boundingBox.GetTop())*heightRatio),
					int(float64(boundingBox.GetRight())*widthRatio),
					int(float64(boundingBox.GetBottom())*heightRatio)),
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
