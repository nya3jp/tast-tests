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
func (p point) toCoord() coords.Point {
	return coords.Point{int(math.Round(p.x)), int(math.Round(p.y))}
}

// readHandwritingFile scans the handwriting file to populate the data into the corresponding structs.
func readHandwritingFile(ctx context.Context, tconn *chrome.TestConn, filePath string, strokeContainer *strokeGroup) error {
	file, err := os.Open(filePath)
	if err != nil {
		return errors.Wrap(err, "failed to read handwriting file")
	}
	defer file.Close()

	// Create a scanner to scan through each line of the input file.
	scanner := bufio.NewScanner(file)

	// Read in the width and height located in the first line of the input file.
	scanner.Scan()
	if err := scanner.Err(); err != nil {
		return errors.Wrap(err, "failed to scan file")
	}
	lineReader := strings.NewReader(scanner.Text())
	_, err = fmt.Fscanf(lineReader, "%f%f", &strokeContainer.width, &strokeContainer.height)
	if err != nil {
		return errors.Wrap(err, "failed to read canvas width and height from handwriting file")
	}

	// Process thea rest of the lines to populate the points into strokes.
	for scanner.Scan() {
		var currentStroke stroke
		// Read in the current line and populate a single stroke.
		lineReader := strings.NewReader(scanner.Text())
		for {
			var currentPoint point
			if _, err = fmt.Fscanf(lineReader, "%f%f", &currentPoint.x, &currentPoint.y); err != nil {
				// If the error is EOF, it means that the scanner reached the end of the line and isn't an actual error.
				if err == io.EOF {
					break
				}
				return errors.Wrap(err, "failed to read points from handwriting file")
			}
			currentStroke.points = append(currentStroke.points, currentPoint)
		}
		if len(currentStroke.points) > 0 {
			strokeContainer.strokes = append(strokeContainer.strokes, currentStroke)
		}
	}
	if err := scanner.Err(); err != nil {
		return errors.Wrap(err, "failed to scan file")
	}

	return nil
}

// scaleHandwritingData scales the data in the structs to fit into the handwriting input.
func scaleHandwritingData(ctx context.Context, tconn *chrome.TestConn, strokeContainer *strokeGroup) error {
	// Find the handwriting canvas location.
	canvas, err := findHandwritingCanvas(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to find handwriting canvas")
	}
	canvasLocation := canvas.Location

	// Scale the width and height of strokeGroup according to the handwriting canvas size.
	scale := math.Min(float64(canvasLocation.Width)*0.6/strokeContainer.width, float64(canvasLocation.Height)*0.6/strokeContainer.height)
	strokeContainer.width *= scale
	strokeContainer.height *= scale

	// Initialise offset values for width and height so that the points are within the canvas.
	widthOffset := float64(canvasLocation.Left) + (float64(canvasLocation.Width)-strokeContainer.width)/2.0
	heightOffset := float64(canvasLocation.Top) + (float64(canvasLocation.Height)-strokeContainer.height)/2.0

	// Process the populated coordinates so that they can be contained with the canvas.
	for i := 0; i < len(strokeContainer.strokes); i++ {
		for j := 0; j < len(strokeContainer.strokes[i].points); j++ {
			// The coordinates are overwritten by the scaled coordinates.
			strokeContainer.strokes[i].points[j].x = strokeContainer.strokes[i].points[j].x*scale + widthOffset
			strokeContainer.strokes[i].points[j].y = strokeContainer.strokes[i].points[j].y*scale + heightOffset
		}
	}

	return nil
}

// drawHandwriting draws the strokes into the handwriting input.
func drawHandwriting(ctx context.Context, tconn *chrome.TestConn, strokeContainer *strokeGroup) error {
	// Draw the strokes into the handwriting input.
	for _, currentStroke := range strokeContainer.strokes {
		currentPoints := currentStroke.points
		for i, currentPoint := range currentPoints {
			// Mouse will be moved to each of the points to draw the stroke.
			// A stroke can have up to 100 points, if a stroke is long enough and uses all 100 points to represent
			// the stroke, it will take 3 seconds (30ms * 100) to draw that stroke.
			if err := mouse.Move(ctx, tconn, currentPoint.toCoord(), 30*time.Millisecond); err != nil {
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
	strokeContainer := &strokeGroup{}

	if err := readHandwritingFile(ctx, tconn, filePath, strokeContainer); err != nil {
		return errors.Wrap(err, "failed to read data from file")
	}

	if err := scaleHandwritingData(ctx, tconn, strokeContainer); err != nil {
		return errors.Wrap(err, "failed to scale the populated data")
	}

	if err := drawHandwriting(ctx, tconn, strokeContainer); err != nil {
		return errors.Wrap(err, "failed to draw handwriting onto the handwriting input")
	}

	return nil
}
