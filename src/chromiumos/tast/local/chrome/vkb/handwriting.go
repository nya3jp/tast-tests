// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vkb

import (
	"bufio"
	"context"
	"fmt"
	"io/ioutil"
	"math"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/mouse"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

// point is a single coordinate on the canvas.
type point struct {
	x float64
	y float64
}

// path contains an array of points that form a stroke.
type path struct {
	points []point
}

// pathGroup contains an array of strokes that form the expectedText.
type pathGroup struct {
	width  float64
	height float64
	paths  []path
}

// findHandwritingCanvas finds the canvas for the handwriting input which will be used to draw the handwriting.
func findHandwritingCanvas(ctx context.Context, tconn *chrome.TestConn) (*ui.Node, error) {
	params := ui.FindParams{
		Role:      ui.RoleTypeCanvas,
		ClassName: "ita-hwt-canvas",
	}
	opts := testing.PollOptions{Timeout: 2 * time.Second, Interval: 200 * time.Millisecond}
	return ui.StableFind(ctx, tconn, params, &opts)
}

// toCoord converts a point in float64 format into a coordinate in int format.
func toCoord(_point point) coords.Point {
	// 0.5 is added to both x and y values as golang discards the fraction when converting from float to int.
	// Adding 0.5 will allow values with fractions over 0.5 to round up instead.
	return coords.Point{int(_point.x + 0.5), int(_point.y + 0.5)}
}

// ReadFileAndDrawHandwriting reads the handwriting file, transforms the points into the correct scale,
// populates the data into the struct, and draws the strokes into the handwriting input.
func ReadFileAndDrawHandwriting(ctx context.Context, tconn *chrome.TestConn, filePath string) error {
	_pathGroup := &pathGroup{}
	file, err := ioutil.ReadFile(filePath)
	if err != nil {
		return errors.Wrap(err, "failed to read handwriting file")
	}

	// Create a scanner to scan through each line of the input file.
	scanner := bufio.NewScanner(strings.NewReader(string(file)))

	// Read in the width and height located in the first line of the input file.
	scanner.Scan()
	lineReader := strings.NewReader(scanner.Text())
	_, err = fmt.Fscanf(lineReader, "%f%f", &_pathGroup.width, &_pathGroup.height)
	if err != nil {
		return errors.Wrap(err, "failed to read canvas width and height from handwriting file")
	}

	// Find the handwriting canvas location.
	canvas, err := findHandwritingCanvas(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to find handwriting canvas")
	}
	canvasLocation := canvas.Location

	// Scale the width and height of pathGroup according to the handwriting canvas size.
	scale := math.Min(float64(canvasLocation.Width)*0.6/_pathGroup.width, float64(canvasLocation.Height)*0.6/_pathGroup.height)
	_pathGroup.width *= scale
	_pathGroup.height *= scale

	// Initialise offset values for width and height so that the points are within the canvas.
	widthOffset := float64(canvasLocation.Left) + (float64(canvasLocation.Width)-_pathGroup.width)/2.0
	heightOffset := float64(canvasLocation.Top) + (float64(canvasLocation.Height)-_pathGroup.height)/2.0

	// Process the rest of the lines to populate the points into the paths.
	for scanner.Scan() {
		var _point point
		var _path path

		// Read in the current line and populate a single path.
		lineReader := strings.NewReader(scanner.Text())
		for {
			_, err = fmt.Fscanf(lineReader, "%f%f", &_point.x, &_point.y)
			if err != nil {
				break
			}
			// Points are scaled as they should all be contained within the canvas.
			_point.x = _point.x*scale + widthOffset
			_point.y = _point.y*scale + heightOffset
			_path.points = append(_path.points, _point)
		}
		if len(_path.points) > 0 {
			_pathGroup.paths = append(_pathGroup.paths, _path)
		}
	}

	// Draw the strokes into the handwriting input.
	for _, _path := range _pathGroup.paths {
		_points := _path.points
		for i, _point := range _points {
			// Mouse will be moved to each of the points to draw the stroke.
			if err := mouse.Move(ctx, tconn, toCoord(_point), 30*time.Millisecond); err != nil {
				return errors.Wrap(err, "failed to move mouse")
			}
			// Left mouse button should be pressed on only the first Point of every path to start the stroke.
			if i == 0 {
				if err := mouse.Press(ctx, tconn, mouse.LeftButton); err != nil {
					return errors.Wrap(err, "failed to click mouse")
				}
			}
		}
		// After going through all the points for a single stroke, release the left mouse button.
		if err := mouse.Release(ctx, tconn, mouse.LeftButton); err != nil {
			return errors.Wrap(err, "failed to release mouse")
		}
	}

	return nil
}
