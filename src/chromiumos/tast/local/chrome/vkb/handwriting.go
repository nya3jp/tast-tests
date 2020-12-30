// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vkb

import (
	"bufio"
	"context"
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/mouse"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

// Point is a single coordinate on the canvas.
type Point struct {
	x float64
	y float64
}

// Path contains an array of Points that form a stroke.
type Path struct {
	Points []Point
}

// PathGroup contains an array of strokes that form the expectedText.
type PathGroup struct {
	width  float64
	height float64
	Paths  []Path
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

// toCoord converts a Point in float64 format into a coordinate in int format.
func toCoord(point Point) coords.Point {
	return coords.Point{int(point.x + 0.5), int(point.y + 0.5)}
}

// TapHandwritingInput changes virtual keyboard to handwriting input layout.
func TapHandwritingInput(ctx context.Context, tconn *chrome.TestConn) error {
	params := ui.FindParams{
		Role:      ui.RoleTypeButton,
		Name:      "switch to handwriting, not compatible with ChromeVox",
		ClassName: "sk icon-key",
	}
	opts := testing.PollOptions{Timeout: 2 * time.Second, Interval: 200 * time.Millisecond}

	return ui.StableFindAndClick(ctx, tconn, params, &opts)
}

// ReadFileAndPopulateData reads the handwriting file, transforms the Points into the correct scale,
// and populates the data into the struct.
func ReadFileAndPopulateData(ctx context.Context, tconn *chrome.TestConn, filename string) (*PathGroup, error) {
	pathGroup := &PathGroup{}
	file, err := os.Open(filename)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read handwriting file")
	}
	defer file.Close()

	// Create a scanner to scan through each line of the input file.
	scanner := bufio.NewScanner(file)

	// Read in the width and height located in the first line of the input file.
	scanner.Scan()
	lineReader := strings.NewReader(scanner.Text())
	_, err = fmt.Fscanf(lineReader, "%f%f", &pathGroup.width, &pathGroup.height)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read canvas width and height from handwriting file")
	}

	// Find the handwriting canvas location.
	canvas, err := findHandwritingCanvas(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find handwriting canvas")
	}
	canvasLocation := canvas.Location

	// Scale the width and height of PathGroup according to the handwriting canvas size.
	scale := math.Min(float64(canvasLocation.Width)*0.6/pathGroup.width, float64(canvasLocation.Height)*0.6/pathGroup.height)
	pathGroup.width *= scale
	pathGroup.height *= scale

	// Initialise offset values for width and height so that the Points are within the canvas.
	widthOffset := float64(canvasLocation.Left) + (float64(canvasLocation.Width)-pathGroup.width)/2.0
	heightOffset := float64(canvasLocation.Top) + (float64(canvasLocation.Height)-pathGroup.height)/2.0

	// Process the rest of the lines to populate the Points into the Paths.
	for scanner.Scan() {
		var point Point
		var path Path

		// Read in the current line and populate a single Path.
		lineReader := strings.NewReader(scanner.Text())
		for {
			_, err = fmt.Fscanf(lineReader, "%f%f", &point.x, &point.y)
			if err != nil {
				break
			}
			// Points are scaled as they should all be contained within the canvas.
			point.x = point.x*scale + widthOffset
			point.y = point.y*scale + heightOffset
			path.Points = append(path.Points, point)
		}
		if len(path.Points) > 0 {
			pathGroup.Paths = append(pathGroup.Paths, path)
		}
	}

	return pathGroup, nil
}

// DrawHandwriting uses the mouse functionality to draw handwriting strokes into the handwriting input.
func DrawHandwriting(ctx context.Context, tconn *chrome.TestConn, pathGroup *PathGroup) error {
	for _, path := range pathGroup.Paths {
		points := path.Points
		for i, point := range points {
			// Mouse will be moved to each of the Points to draw the stroke.
			if err := mouse.Move(ctx, tconn, toCoord(point), 30*time.Millisecond); err != nil {
				return errors.Wrap(err, "failed to move mouse")
			}
			// Left mouse button should be pressed on only the first Point of every Path to start the stroke.
			if i == 0 {
				if err := mouse.Press(ctx, tconn, mouse.LeftButton); err != nil {
					return errors.Wrap(err, "failed to click mouse")
				}
			}
		}
		// After going through all the Points for a single stroke, release the left mouse button.
		if err := mouse.Release(ctx, tconn, mouse.LeftButton); err != nil {
			return errors.Wrap(err, "failed to release mouse")
		}
	}
	return nil
}
