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

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/mouse"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

// structs required to unmarshal the SVG file.
type svg struct {
	Defs defs `xml:"defs"`
}

type defs struct {
	Paths []path `xml:"path"`
}

type path struct {
	D string `xml:"d,attr"`
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

// newStrokeGroup unmarshals the svg file and returns a strokeGroup with the populated data.
// n is the number of desired points per stroke.
// Detailed explanation of the algorithm can be found in go/tast-handwriting-svg-parsing.
func newStrokeGroup(filePath string, n int) (*strokeGroup, error) {
	// Open the file that needs to be read.
	file, err := os.Open(filePath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read SVG file")
	}
	defer file.Close()

	// Populate the svg struct with the data in the SVG file.
	byteValue, _ := ioutil.ReadAll(file)
	svgFile := &svg{}
	if err := xml.Unmarshal(byteValue, svgFile); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal SVG file")
	}

	// Create an instance of strokeGroup.
	sg := &strokeGroup{}

	// Populate the strokeGroup struct with the strokes in the svg struct.
	paths := svgFile.Defs.Paths
	for _, path := range paths {
		var s stroke
		// Convert the path string into an array of points by removing first character which will always
		// be 'M' and splitting the string using 'L'.
		coords := strings.Split(path.D[1:], "L")

		// Quotient and remainder are used to calculate the number of points required in between the original points.
		length := len(coords)
		quotient, remainder := (n-length)/(length-1), (n-length)%(length-1)

		for i, coord := range coords {
			var p point
			// Populate a single point into the point struct and append to the current stroke.
			fmt.Sscanf(coord, "%f %f", &p.x, &p.y)

			if i != 0 {
				// Get the previous points to calculate the change in x and y values between two points.
				prev := s.points[len(s.points)-1]

				// Define the number of points required in between two points using quotient and remainder.
				pointsInBetween := quotient
				if remainder > 0 {
					pointsInBetween++
					remainder--
				}

				// Define the required change in x and y for each new point in between the two original points.
				dx, dy := (p.x-prev.x)/(float64(pointsInBetween+1)), (p.y-prev.y)/(float64(pointsInBetween+1))

				// Append the newly calculated points into points.
				for j := 1; j < pointsInBetween+1; j++ {
					var newP point
					newP.x, newP.y = prev.x+dx*float64(j), prev.y+dy*float64(j)
					s.points = append(s.points, newP)
				}
			}
			s.points = append(s.points, p)
		}
		if len(s.points) > 0 {
			sg.strokes = append(sg.strokes, s)
		}
	}

	return sg, nil
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
	// Number of desired points per stroke.
	const n = 100

	// Scan the handwriting file and return a strokeGroup with the populated data.
	sg, err := newStrokeGroup(filePath, n)
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
