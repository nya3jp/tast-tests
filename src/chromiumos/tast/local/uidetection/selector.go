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

// Location is a rectangle containing a matching UI element.
type Location struct {
	TopLeft     coords.Point
	BottomRight coords.Point
}

// Selector represents a data structure that consists of arguments to find
// a UI element.
type Selector struct {
	// The request used to construct the selector.
	request *pb.DetectionRequest
	// The response used to construct the selector.
	boundingBoxes []*Location
	// Descriptor for the selector.
	desc string
	nth  int
}

func newSelector() *Selector {
	return &Selector{}
}

func newFromRequest(r *pb.DetectionRequest, d string) *Selector {
	return &Selector{
		request:       r,
		boundingBoxes: nil,
		desc:          d,
		nth:           -1,
	}
}

// copy returns a copy of the input Finder.
// It copies all of the keys/values in attributes and state individually.
func (s *Selector) copy() *Selector {
	c := newSelector()
	c.request = s.request
	c.boundingBoxes = s.boundingBoxes
	c.desc = s.desc
	c.nth = s.nth
	return c
}

// First enables the selector to choose the first match of a UI element
// if there are multiple matches.
func (s *Selector) First() *Selector {
	c := s.copy()
	c.nth = 0
	return c
}

// Nth enables the selector to choose the n-th match of a UI element.
// if there are multiple matches.
func (s *Selector) Nth(nth int) *Selector {
	c := s.copy()
	c.nth = nth
	return c
}

// resolve resolves the UI detection request and stores the bounding boxes
// of the matching elements.
// This provides flexibility to users to decide when to resolve the detection,
// e.g. when it depends on certain conditions.
func (s *Selector) resolve(ctx context.Context, d *uiDetector) error {
	response, err := d.sendDetectionRequest(ctx, s.request)
	if err != nil {
		return errors.Wrap(err, "failed to resolve the UI detection request")
	}

	s.boundingBoxes = []*Location{}
	for _, boundingBox := range response.BoundingBoxes {
		s.boundingBoxes = append(
			s.boundingBoxes,
			&Location{
				TopLeft:     coords.NewPoint(int(boundingBox.GetLeft()), int(boundingBox.GetTop())),
				BottomRight: coords.NewPoint(int(boundingBox.GetRight()), int(boundingBox.GetBottom())),
			},
		)
	}
	return nil
}

func (s *Selector) location() (*Location, error) {
	if s.boundingBoxes == nil {
		return nil, errors.New("the selector is not resolved")
	}
	numMatches := len(s.boundingBoxes)
	switch {
	case numMatches == 0:
		return nil, errors.New("no matching elements found")
	case numMatches == 1:
		return s.boundingBoxes[0], nil
	default:
		if s.nth < 0 {
			return nil, errors.New("there are multiple matches. If it is expected, consider using First() or Nth()")
		}
		if s.nth > numMatches-1 {
			return nil, errors.Errorf("find %d elements, but want the %d-th one", numMatches, s.nth)
		}
		return s.boundingBoxes[s.nth], nil
	}
}
