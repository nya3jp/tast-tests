// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package coords keeps coordinates-related structs and their utilities for the
// UI system.
package coords

import (
	"fmt"
	"math"
)

// Point represents a location.
type Point struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// NewPoint creates a new Point instance for given x,y coordinates.
func NewPoint(x, y int) Point {
	return Point{X: x, Y: y}
}

// String returns the string representation of Point.
func (p Point) String() string {
	return fmt.Sprintf("(%d, %d)", p.X, p.Y)
}

// Size represents a size of a region.
type Size struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// NewSize creates a new Size instance for given width,height.
func NewSize(w, h int) Size {
	return Size{Width: w, Height: h}
}

// Rect represents a rectangular region.
type Rect struct {
	Left   int `json:"left"`
	Top    int `json:"top"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// NewRect creates a new Rect instance for given x, y width, and height.
func NewRect(x, y, w, h int) Rect {
	return Rect{Left: x, Top: y, Width: w, Height: h}
}

// NewRectLTRB creates a new rect instance for left, top, right, and bottom
// coordinates.
func NewRectLTRB(l, t, r, b int) Rect {
	return Rect{Left: l, Top: t, Width: r - l, Height: b - t}
}

// String returns the string representation of Rect.
func (r Rect) String() string {
	return fmt.Sprintf("(%d, %d) - (%d x %d)", r.Left, r.Top, r.Width, r.Height)
}

// CenterPoint returns the location of the center of the rectangle.
func (r Rect) CenterPoint() Point {
	return Point{X: r.Left + r.Width/2, Y: r.Top + r.Height/2}
}

// Empty returns true if the r is zero-value.
func (r Rect) Empty() bool {
	return r == Rect{}
}

// ConvertBoundsFromDpToPx converts the given bounds in DP to pixles based on the given device scale factor.
func ConvertBoundsFromDpToPx(bounds Rect, dsf float64) Rect {
	return Rect{
		Left:   int(math.Round(float64(bounds.Left) * dsf)),
		Top:    int(math.Round(float64(bounds.Top) * dsf)),
		Width:  int(math.Round(float64(bounds.Width) * dsf)),
		Height: int(math.Round(float64(bounds.Height) * dsf))}
}
