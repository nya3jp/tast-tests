// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package uidetection

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/coords"
	pb "chromiumos/tast/local/uidetection/api"
)

// Location represents a matching UI element.
type Location struct {
	// Rectangle of the location.
	coords.Rect
	// Text associated with the element, if any.
	Text string
}

func locationFromPBBox(boundingBox *pb.BoundingBox) *Location {
	return &Location{
		Rect: coords.Rect{
			Left:   int(boundingBox.GetLeft()),
			Top:    int(boundingBox.GetTop()),
			Width:  int(boundingBox.GetRight()) - int(boundingBox.GetLeft()),
			Height: int(boundingBox.GetBottom()) - int(boundingBox.GetTop()),
		},
		Text: boundingBox.GetText(),
	}
}

// Finder represents a data structure that consists of arguments to find
// a UI element.
type Finder struct {
	// The request used to construct the finder.
	request *pb.DetectionRequest
	// The response used to construct the finder.
	boundingBoxes []*Location
	// Descriptor for the finder.
	desc string
	nth  int
}

func newFinder() *Finder {
	return &Finder{}
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
// This provides flexibility to users to decide when to resolve the detection,
// e.g. when it depends on certain conditions.
func (s *Finder) resolve(ctx context.Context, d *uiDetector) error {
	response, err := d.sendDetectionRequest(ctx, s.request)
	if err != nil {
		return errors.Wrap(err, "failed to resolve the UI detection request")
	}

	s.boundingBoxes = []*Location{}
	for _, boundingBox := range response.BoundingBoxes {
		s.boundingBoxes = append(s.boundingBoxes, locationFromPBBox(boundingBox))
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
		return nil, errors.New("no matching elements found")
	case numMatches == 1:
		if s.nth > 0 {
			return nil, errors.Errorf("find only one element, but want the %d-th one", s.nth)
		}
		return s.boundingBoxes[0], nil
	default: // case numMatches > 1.
		if s.nth < 0 {
			return nil, errors.New("there are multiple matches. If it is expected, consider using First() or Nth()")
		}
		if s.nth > numMatches-1 {
			return nil, errors.Errorf("find %d elements, but want the %d-th one", numMatches, s.nth)
		}
		return s.boundingBoxes[s.nth], nil
	}
}
