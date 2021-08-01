// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vkb

import (
	"context"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

// HandwritingContext represents a context for handwriting.
type HandwritingContext struct {
	VirtualKeyboardContext
	isLongForm bool
}

// NewHandwritingContext creates a new context for handwriting.
func (vkbCtx *VirtualKeyboardContext) NewHandwritingContext(ctx context.Context) (*HandwritingContext, error) {
	hwCtx := &HandwritingContext{
		VirtualKeyboardContext: *vkbCtx,
		isLongForm:             false,
	}

	testing.Poll(ctx, func(ctx context.Context) error {
		if err := hwCtx.ui.Exists(NodeFinder.HasClass("lf-keyboard"))(ctx); err != nil {
			return err
		}
		hwCtx.isLongForm = true
		return nil
	}, &testing.PollOptions{
		Timeout: 2 * time.Second})

	return hwCtx, nil
}

// Structs required to unmarshal the SVG file.
type svg struct {
	Defs defs `xml:"defs"`
}

type defs struct {
	Paths []path `xml:"path"`
}

type path struct {
	D string `xml:"d,attr"`
}

// readSvg reads the SVG file and populates the corresponding structs.
func readSvg(filePath string) (*svg, error) {
	// Open the file that needs to be read.
	file, err := os.Open(filePath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open SVG file at %s", filePath)
	}
	defer file.Close()

	// Populate the svg struct with the data in the SVG file.
	byteValue, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read SVG file")
	}
	svgFile := &svg{}
	if err := xml.Unmarshal(byteValue, svgFile); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal SVG file")
	}

	return svgFile, nil
}

// point is a single coordinate on the canvas.
type point struct {
	x float64
	y float64
}

// toCoordsPoint converts a point in float64 format into a coordinate in int format.
func (p point) toCoordsPoint() coords.Point {
	return coords.Point{X: int(math.Round(p.x)), Y: int(math.Round(p.y))}
}

// linearInterpolation calculates the new point's x and y values using linear interpolation.
func linearInterpolation(p1, p2 point, pointsInBetween, i int) point {
	t := float64(i) / float64(pointsInBetween+1)
	return point{x: (1-t)*p1.x + t*p2.x, y: (1-t)*p1.y + t*p2.y}
}

// stroke contains an array of points that form a stroke.
type stroke struct {
	points []point
}

// newStroke creates a stroke from the path data.
func newStroke(path *path, n int) *stroke {
	s := &stroke{}

	// Get the extracted strokes from the path string in the SVG struct.
	extractedStroke := parseStroke(path)

	// Quotient and remainder are used to calculate the number of points required in between the original points.
	length := len(extractedStroke.points)
	quotient, remainder := (n-length)/(length-1), (n-length)%(length-1)

	for i, p := range extractedStroke.points {
		if i != 0 {
			// Get the previous points to calculate the change in x and y values between two points.
			prev := s.points[len(s.points)-1]

			// Define the number of points required in between two points using quotient and remainder.
			pointsInBetween := quotient
			if remainder > 0 {
				pointsInBetween++
				remainder--
			}

			// Append the newly calculated points into points.
			for j := 0; j < pointsInBetween; j++ {
				newP := linearInterpolation(prev, p, pointsInBetween, j+1)
				s.points = append(s.points, newP)
			}
		}
		s.points = append(s.points, p)
	}

	return s
}

// parseStroke transforms the SVG path data from string to a stroke.
func parseStroke(path *path) *stroke {
	s := &stroke{}

	// Convert the path string into an array of points by removing first character which will always
	// be 'M' and splitting the string using 'L'.
	coords := strings.Split(path.D[1:], "L")

	for _, coord := range coords {
		var p point
		fmt.Sscanf(coord, "%f %f", &p.x, &p.y)
		s.points = append(s.points, p)
	}

	return s
}

// strokeGroup contains an array of strokes that form the text that will be drawn into the handwriting input.
type strokeGroup struct {
	width   float64
	height  float64
	strokes []stroke
}

// newStrokeGroup unmarshals the SVG file and returns a strokeGroup with the populated data.
// n is the number of desired points per stroke.
// Detailed explanation of the algorithm can be found in go/tast-handwriting-svg-parsing.
func newStrokeGroup(svgFile *svg, n int) *strokeGroup {
	sg := &strokeGroup{}

	// Populate the strokeGroup struct with the strokes in the svg struct.
	for _, path := range svgFile.Defs.Paths {
		s := newStroke(&path, n)
		if len(s.points) > 0 {
			sg.strokes = append(sg.strokes, *s)
		}
	}

	return sg
}

// getMinMax returns the min x, max x, min y, max y values found within strokeGroup to calculate the
// width and height of the handwriting text.
func getMinMax(sg *strokeGroup) (*point, *point) {
	// Initialise the min points to infinity, and the max points to 0. Max points will never be smaller than 0
	// as negative coordinates are not used when creating SVG files.
	minPoint, maxPoint := &point{math.Inf(1), math.Inf(1)}, &point{0, 0}
	for _, s := range sg.strokes {
		for _, p := range s.points {
			minPoint.x, minPoint.y = math.Min(minPoint.x, p.x), math.Min(minPoint.y, p.y)
			maxPoint.x, maxPoint.y = math.Max(maxPoint.x, p.x), math.Max(maxPoint.y, p.y)
		}
	}

	return minPoint, maxPoint
}

// scale scales the data in the structs to fit into the handwriting input.
func (sg *strokeGroup) scale(canvasLoc coords.Rect) {
	// Get the min and max of x and y points to identify the size of the handwriting.
	minPoint, maxPoint := getMinMax(sg)
	minX, minY := minPoint.x, minPoint.y
	maxX, maxY := maxPoint.x, maxPoint.y

	// Define the SVG file's width and height.
	width := math.Floor(maxX - minX)
	height := math.Ceil(maxY - minY)

	// Initialise the width and height of strokeGroup which will be scaled.
	sg.width = 1.0
	sg.height = height / width

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
			sg.strokes[i].points[j].x = (sg.strokes[i].points[j].x-minX)/width*scale + widthOffset
			sg.strokes[i].points[j].y = (sg.strokes[i].points[j].y-minY)/width*scale + heightOffset
		}
	}
}

// drawStrokes draws the strokes into the handwriting input.
func drawStrokes(ctx context.Context, tconn *chrome.TestConn, sg *strokeGroup) error {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 500*time.Millisecond)
	defer cancel()
	defer func(ctx context.Context) {
		if err := mouse.Release(tconn, mouse.LeftButton)(ctx); err != nil {
			testing.ContextLogf(ctx, "Failed to release mouse: %s", err.Error())
		}
	}(cleanupCtx)
	// Draw the strokes into the handwriting input.
	for _, s := range sg.strokes {
		for i, p := range s.points {
			// Mouse will be moved to each of the points to draw the stroke.
			// A stroke can have up to 50 points, if a stroke is long enough and uses all 50 points to represent
			// the stroke, it will take 2 seconds (40ms * 50) to draw that stroke.
			if err := mouse.Move(tconn, p.toCoordsPoint(), 40*time.Millisecond)(ctx); err != nil {
				return errors.Wrap(err, "failed to move mouse")
			}
			// Left mouse button should be pressed on only the first point of every stroke to start the stroke.
			if i == 0 {
				if err := mouse.Press(tconn, mouse.LeftButton)(ctx); err != nil {
					return errors.Wrap(err, "failed to click mouse")
				}
			}
		}
		// After going through all the points for a single stroke, release the left mouse button.
		if err := mouse.Release(tconn, mouse.LeftButton)(ctx); err != nil {
			return errors.Wrap(err, "failed to release mouse")
		}
	}

	return nil
}

// DrawStrokesFromFile returns an action reading the handwriting file, transforming the points into the correct scale,
// populates the data into the struct, and drawing the strokes into the handwriting input.
func (hwCtx *HandwritingContext) DrawStrokesFromFile(filePath string) uiauto.Action {
	return func(ctx context.Context) error {
		// Number of points we would like per stroke.
		const n = 50

		// Read and unmarshal the SVG file into the corresponding structs.
		svgFile, err := readSvg(filePath)
		if err != nil {
			return errors.Wrap(err, "failed to read data from file")
		}

		// Scan the handwriting file and return a strokeGroup with the populated data.
		sg := newStrokeGroup(svgFile, n)

		// Find the handwriting canvas location.
		hwCanvasFinder := NodeFinder.Role(role.Canvas)
		loc, err := hwCtx.ui.Location(ctx, hwCanvasFinder)
		if err != nil {
			return errors.Wrapf(err, "failed to get location of %v", hwCanvasFinder)
		}

		// Scale the handwriting data in the structs to fit the handwriting input.
		sg.scale(*loc)

		// Draw the handwriting into the handwriting input.
		if err := drawStrokes(ctx, hwCtx.tconn, sg); err != nil {
			return errors.Wrap(err, "failed to draw handwriting onto the handwriting input")
		}

		return nil
	}
}

// ClearHandwritingCanvas returns an action that clears the handwriting canvas.
// TODO(b/189277286): Add support to check whether a handwriting canvas is clear for a non-longform canvas.
func (hwCtx *HandwritingContext) ClearHandwritingCanvas() uiauto.Action {
	if !hwCtx.isLongForm {
		return hwCtx.TapKey("backspace")
	}
	undoKey := KeyFinder.Name("undo")
	waitForBackspaceAction := hwCtx.ui.WithTimeout(500 * time.Millisecond).WaitUntilExists(KeyFinder.Name("backspace"))

	// Undo key remains on the keyboard if the canvas is not clear in longform canvas.
	return hwCtx.ui.IfSuccessThen(
		hwCtx.ui.WithTimeout(time.Second).WaitUntilExists(undoKey),
		hwCtx.ui.LeftClickUntil(undoKey, waitForBackspaceAction))

}

// WaitForHandwritingEngineReady returns an action that waits for the handwriting engine to become ready.
func (hwCtx *HandwritingContext) WaitForHandwritingEngineReady(checkHandwritingEngineReady uiauto.Action) uiauto.Action {
	return uiauto.NamedAction("Wait for handwriting engine ready",
		hwCtx.ui.WithTimeout(time.Minute).Retry(10, checkHandwritingEngineReady))
}
