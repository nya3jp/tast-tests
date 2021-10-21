// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package uidetection

import (
	pb "chromiumos/tast/local/uidetection/api"
)

// Selector represents a data structure that consists of arguments to find
// a UI element.
type Selector struct {
	// The request used to construct the selector, it is null if resolved.
	request *pb.DetectionRequest
	// The response used to construct the selector, it is null if not resolved.
	response *pb.UiDetectionResponse
	// Whether the operation by the selector has been resolved.
	// If resolved, request will be null and response will be non-null,
	// and vice versa if not resolved.
	resolved bool
	first    bool
	nth      int
}

func newSelector() *Selector {
	return &Selector{}
}

func newFromRequest(r *pb.DetectionRequest) *Selector {
	return &Selector{
		request: r,
	}
}

func newFromResponse(r *pb.UiDetectionResponse) *Selector {
	return &Selector{
		response: r,
		resolved: true,
	}
}

// copy returns a copy of the input Finder.
// It copies all of the keys/values in attributes and state individually.
func (s *Selector) copy() *Selector {
	c := newSelector()
	c.request = s.request
	c.response = s.response
	c.resolved = s.resolved
	c.first = s.first
	c.nth = s.nth
	return c
}

// First enables the selector to choose the first match of a UI element
// if there are multiple matches.
func (s *Selector) First() *Selector {
	c := s.copy()
	s.first = true
	return c
}

// First enables the selector to choose the n-th match of a UI element.
// if there are multiple matches.
func (s *Selector) Nth(nth int) *Selector {
	c := s.copy()
	s.nth = nth
	return c
}
