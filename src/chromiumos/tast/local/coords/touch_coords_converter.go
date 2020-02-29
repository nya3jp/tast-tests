// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package coords

import "chromiumos/tast/local/input"

// TouchCoordConverter manages the conversion between locations in DIP and
// the TouchCoord of the touchscreen.
type TouchCoordConverter struct {
	ScaleX float64
	ScaleY float64
}

// NewTouchCoordConverter creates a new TouchCoordConverter instance for the
// given size and touchscreen.
func NewTouchCoordConverter(size Size, tsew *input.TouchscreenEventWriter) *TouchCoordConverter {
	return &TouchCoordConverter{
		ScaleX: float64(tsew.Width()) / float64(size.Width),
		ScaleY: float64(tsew.Height()) / float64(size.Height),
	}
}

// ConvertLocation converts a location to TouchCoord.
func (tcc *TouchCoordConverter) ConvertLocation(l Point) (x, y input.TouchCoord) {
	return input.TouchCoord(tcc.ScaleX * float64(l.X)), input.TouchCoord(tcc.ScaleY * float64(l.Y))
}
