// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vkb

import (
	"bufio"
	"context"
	"fmt"
	"io"
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

// point is a single coordinate on the canvas.
type point struct {
	x float64
	y float64
}

// toCoordsPoint converts a point in float64 format into a coordinate in int format.
func (p point) toCoordsPoint() coords.Point {
	return coords.Point{X: int(math.Round(p.x)), Y: int(math.Round(p.y))}
}

// stroke contains an array of points that form a stroke.
type stroke struct {
	points []point
}

// strokeGroup contains an array of strokes that form the text that will be drawn into the handwriting input.
type strokeGroup struct {
	width   float64
	height  float64
	strokes []stroke
}

// newStrokeGroup scans the handwriting file and returns a strokeGroup with the populated data.
func newStrokeGroup(filePath string) (*strokeGroup, error) {
	// Open the file that needs to be read.
	file, err := os.Open(filePath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read handwriting file")
	}
	defer file.Close()

	// Create an instance of strokeGroup.
	sg := &strokeGroup{}

	// Create a scanner to scan through each line of the input file.
	scanner := bufio.NewScanner(file)

	// Read in the width and height located in the first line of the input file.
	scanner.Scan()
	if err := scanner.Err(); err != nil {
		return nil, errors.Wrap(err, "failed to scan file")
	}
	lineReader := strings.NewReader(scanner.Text())
	if _, err := fmt.Fscanf(lineReader, "%f %f", &sg.width, &sg.height); err != nil {
		return nil, errors.Wrap(err, "failed to read canvas width and height from handwriting file")
	}

	// Process the rest of the lines to populate the points into strokes.
	for scanner.Scan() {
		var s stroke
		// Read in the current line and populate a single stroke.
		lineReader := strings.NewReader(scanner.Text())
		for {
			var p point
			if _, err := fmt.Fscanf(lineReader, "%f %f", &p.x, &p.y); err != nil {
				// If the error is EOF, it means that the scanner reached the end of the line and isn't an actual error.
				if err == io.EOF {
					break
				}
				return nil, errors.Wrap(err, "failed to read points from handwriting file")
			}
			s.points = append(s.points, p)
		}
		if len(s.points) > 0 {
			sg.strokes = append(sg.strokes, s)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, errors.Wrap(err, "failed to scan file")
	}

	return sg, nil
}

// scale scales the data in the structs to fit into the handwriting input.
func (sg *strokeGroup) scale(canvasLoc coords.Rect) {
	// A multiplier that scales the points to make the handwriting smaller than the canvas.
	const multiplier = 0.6

	// Scale the width and height of strokeGroup according to the handwriting canvas size.
	scale := math.Min(float64(canvasLoc.Width)/sg.width, float64(canvasLoc.Height)/sg.height) * multiplier
	sg.width *= scale
	sg.height *= scale

	// Initialise offset values for width and height so that the points are within the canvas.
	widthOffset := float64(canvasLoc.Left) + (float64(canvasLoc.Width)-sg.width)/2.0
	heightOffset := float64(canvasLoc.Top) + (float64(canvasLoc.Height)-sg.height)/2.0

	// Process the populated coordinates so that they can be contained with the canvas.
	for i := 0; i < len(sg.strokes); i++ {
		for j := 0; j < len(sg.strokes[i].points); j++ {
			// The coordinates are overwritten by the scaled coordinates.
			sg.strokes[i].points[j].x = sg.strokes[i].points[j].x*scale + widthOffset
			sg.strokes[i].points[j].y = sg.strokes[i].points[j].y*scale + heightOffset
		}
	}
}

// findHandwritingCanvas finds the canvas for the handwriting input which will be used to draw the handwriting.
func findHandwritingCanvas(ctx context.Context, tconn *chrome.TestConn) (*ui.Node, error) {
	params := ui.FindParams{
		Role:      ui.RoleTypeCanvas,
		ClassName: "ita-hwt-canvas",
	}
	opts := testing.PollOptions{Timeout: 2 * time.Second}
	return ui.StableFind(ctx, tconn, params, &opts)
}

// drawHandwriting draws the strokes into the handwriting input.
func drawHandwriting(ctx context.Context, tconn *chrome.TestConn, sg *strokeGroup) error {
	// Draw the strokes into the handwriting input.
	for _, s := range sg.strokes {
		for i, p := range s.points {
			// Mouse will be moved to each of the points to draw the stroke.
			// A stroke can have up to 100 points, if a stroke is long enough and uses all 100 points to represent
			// the stroke, it will take 3 seconds (30ms * 100) to draw that stroke.
			if err := mouse.Move(ctx, tconn, p.toCoordsPoint(), 30*time.Millisecond); err != nil {
				return errors.Wrap(err, "failed to move mouse")
			}
			// Left mouse button should be pressed on only the first point of every stroke to start the stroke.
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

// DrawHandwritingFromFile reads the handwriting file, transforms the points into the correct scale,
// populates the data into the struct, and draws the strokes into the handwriting input.
func DrawHandwritingFromFile(ctx context.Context, tconn *chrome.TestConn, filePath string) error {
	// Scan the handwriting file and return a strokeGroup with the populated data.
	sg, err := newStrokeGroup(filePath)
	if err != nil {
		return errors.Wrap(err, "failed to read data from file")
	}

	// Find the handwriting canvas location.
	canvas, err := findHandwritingCanvas(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to find handwriting canvas")
	}

	// Scale the handwriting data in the structs to fit the handwriting input.
	sg.scale(canvas.Location)

	// Draw the handwriting into the handwriting input.
	if err := drawHandwriting(ctx, tconn, sg); err != nil {
		return errors.Wrap(err, "failed to draw handwriting onto the handwriting input")
	}

	return nil
}
