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

// In returns whether the point is contained within r.
func (p Point) In(r Rect) bool {
	return r.Left <= p.X && r.Top <= p.Y && r.Bottom() >= p.Y && r.Right() >= p.X
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

// Equals returns true if the Rect equals another one.
func (r Rect) Equals(r2 Rect) bool {
	return r.Left == r2.Left && r.Top == r2.Top && r.Width == r2.Width && r.Height == r2.Height
}

// Right returns the x-value of the right edge of the rectangle.
func (r Rect) Right() int {
	return r.Left + r.Width
}

// Bottom returns the y-value of the bottom edge of the rectangle.
func (r Rect) Bottom() int {
	return r.Top + r.Height
}

// CenterX returns the x-value of the center point of the rectangle.
func (r Rect) CenterX() int {
	return r.Left + r.Width/2
}

// CenterY returns the y-value of the center point of the rectangle.
func (r Rect) CenterY() int {
	return r.Top + r.Height/2
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

// BottomCenter returns the center location of the bottom edge of the rectangle.
func (r Rect) BottomCenter() Point {
	return Point{X: r.Left + r.Width/2, Y: r.Top + r.Height}
}

// LeftCenter returns the center location of the left edge of the rectangle.
func (r Rect) LeftCenter() Point {
	return Point{X: r.Left, Y: r.Top + r.Height/2}
}

// RightCenter returns the center location of the right edge of the rectangle.
func (r Rect) RightCenter() Point {
	return Point{X: r.Left + r.Width, Y: r.Top + r.Height/2}
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

// Contains returns whether `other` is a rectangle contained within r.
// A rectangle is considered to contain itself.
func (r Rect) Contains(other Rect) bool {
	return r.Left <= other.Left && r.Top <= other.Top && r.Bottom() >= other.Bottom() && r.Right() >= other.Right()
}

func min(x, y int) int {
	if x < y {
		return x
	}
	return y
}

func max(x, y int) int {
	if x < y {
		return y
	}
	return x
}

// Intersection returns the intersection of two rectangles, or an empty
// rectangle if they don't intersect.
func (r Rect) Intersection(other Rect) Rect {
	res := NewRectLTRB(
		max(r.Left, other.Left),
		max(r.Top, other.Top),
		min(r.Right(), other.Right()),
		min(r.Bottom(), other.Bottom()))
	if res.Width < 0 || res.Height < 0 {
		return Rect{}
	}
	return res
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

// convertBounds is used by ConvertBoundsFromDPToPX and ConvertBoundsFromPXToDP.
func convertBounds(bounds Rect, factor float64) Rect {
	return Rect{
		Left:   int(math.Round(float64(bounds.Left) * factor)),
		Top:    int(math.Round(float64(bounds.Top) * factor)),
		Width:  int(math.Round(float64(bounds.Width) * factor)),
		Height: int(math.Round(float64(bounds.Height) * factor))}
}

// ConvertBoundsFromDPToPX converts the given bounds in dips to pixels based on the given device
// scale factor. The converted values of Left, Top, Width, and Height are rounded.
func ConvertBoundsFromDPToPX(bounds Rect, dsf float64) Rect {
	return convertBounds(bounds, dsf)
}

// ConvertBoundsFromPXToDP converts the given bounds in pixels to dips based on the given device
// scale factor. The converted values of Left, Top, Width, and Height are rounded.
func ConvertBoundsFromPXToDP(bounds Rect, dsf float64) Rect {
	return convertBounds(bounds, 1.0/dsf)
}

// CompareBoundsWithMargin returns true if the given two bounds have the same value allowing the same margin
// in all directions.
func CompareBoundsWithMargin(a, b Rect, margin int) bool {
	return CompareBoundsWithMargins(a, b, margin, margin, margin, margin)
}

// CompareBoundsWithMargins returns true if the given two bounds have the same value allowing the provided margins.
func CompareBoundsWithMargins(a, b Rect, ml, mt, mr, mb int) bool {
	return math.Abs(float64(a.Top-b.Top)) <= float64(mt) &&
		math.Abs(float64(a.Bottom()-b.Bottom())) <= float64(mb) &&
		math.Abs(float64(a.Left-b.Left)) <= float64(ml) &&
		math.Abs(float64(a.Right()-b.Right())) <= float64(mr)
}
