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

// Equals returns true if the point equals another one.
func (p Point) Equals(p2 Point) bool {
	return p.X == p2.X && p.Y == p2.Y
}

// Add returns the addition of two Points.
func (p Point) Add(p2 Point) Point {
	return Point{p.X + p2.X, p.Y + p2.Y}
}

// Sub returns the subtraction of two Points.
func (p Point) Sub(p2 Point) Point {
	return Point{p.X - p2.X, p.Y - p2.Y}
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

// String returns the string representation of Size.
func (s Size) String() string {
	return fmt.Sprintf("(%d x %d)", s.Width, s.Height)
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

// TopLeft returns the location of the top left of the rectangle.
func (r Rect) TopLeft() Point {
	return Point{X: r.Left, Y: r.Top}
}

// TopRight returns the location of the top right of the rectangle.
func (r Rect) TopRight() Point {
	return Point{X: r.Left + r.Width, Y: r.Top}
}

// BottomLeft returns the location of the bottom left of the rectangle.
func (r Rect) BottomLeft() Point {
	return Point{X: r.Left, Y: r.Top + r.Height}
}

// BottomRight returns the location of the bottom right of the rectangle.
func (r Rect) BottomRight() Point {
	return Point{X: r.Left + r.Width, Y: r.Top + r.Height}
}

// CenterPoint returns the location of the center of the rectangle.
func (r Rect) CenterPoint() Point {
	return Point{X: r.Left + r.Width/2, Y: r.Top + r.Height/2}
}

// Empty returns true if the r is zero-value.
func (r Rect) Empty() bool {
	return r == Rect{}
}

// Size returns the size of the rect.
func (r Rect) Size() Size {
	return Size{Width: r.Width, Height: r.Height}
}

// WithInset returns a new Rect inset by the given amounts. If insetting would cause the rectangle to
// have negative area, instead an empty rectangle with the same CenterPoint is returned.
// Note that dw and dh may be negative to outset a rectangle.
func (r Rect) WithInset(dw, dh int) Rect {
	dw2 := dw * 2
	if dw2 > r.Width {
		dw2 = r.Width
	}
	dh2 := dh * 2
	if dh2 > r.Height {
		dh2 = r.Height
	}
	return NewRect(r.Left+dw2/2, r.Top+dh2/2, r.Width-dw2, r.Height-dh2)
}

// WithOffset returns a new Rect offset by the given amounts.
func (r Rect) WithOffset(dl, dt int) Rect {
	return NewRect(r.Left+dl, r.Top+dt, r.Width, r.Height)
}

// ConvertBoundsFromDpToPx converts the given bounds in DP to pixles based on the given device scale factor.
func ConvertBoundsFromDpToPx(bounds Rect, dsf float64) Rect {
	return Rect{
		Left:   int(math.Round(float64(bounds.Left) * dsf)),
		Top:    int(math.Round(float64(bounds.Top) * dsf)),
		Width:  int(math.Round(float64(bounds.Width) * dsf)),
		Height: int(math.Round(float64(bounds.Height) * dsf))}
}
