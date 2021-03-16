// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vkb

import (
	"io/ioutil"
	"os"
	"reflect"
	"testing"

	"chromiumos/tast/local/coords"
)

func TestNewStrokeGroup(t *testing.T) {
	want := &strokeGroup{
		width:  0.0,
		height: 0.0,
		strokes: []stroke{
			{
				points: []point{
					{
						x: 10.0,
						y: 10.0,
					},
					point{
						x: 15.0,
						y: 15.0,
					},
					point{
						x: 20.0,
						y: 20.0,
					},
					point{
						x: 25.0,
						y: 25.0,
					},
					point{
						x: 30.0,
						y: 30.0,
					},
				},
			},
		},
	}

	file, err := ioutil.TempFile("", "handwriting_test_")
	if err != nil {
		t.Fatal("TempFile() failed: ", err)
	}
	defer os.Remove(file.Name())
	defer file.Close()

	if _, err := file.Write([]byte(`<svg><defs><path d="M10 10L20 20L30 30"></path></defs></svg>`)); err != nil {
		t.Fatal("Write() failed: ", err)
	}

	svgFile, err := readSvg(file.Name())
	if err != nil {
		t.Fatal("readSvg() failed", err)
	}

	got := newStrokeGroup(svgFile, 5)

	if !reflect.DeepEqual(got, want) {
		t.Errorf("newStrokeGroup() = %+v; want %+v", got, want)
	}
}

func TestScale(t *testing.T) {
	want := &strokeGroup{
		width:  141.0,
		height: 141.0,
		strokes: []stroke{
			{
				points: []point{
					{
						x: 441.5,
						y: 512.0,
					},
					{
						x: 586.025,
						y: 649.475,
					},
				},
			},
		},
	}

	canvasLoc := coords.Rect{
		Left:   97,
		Top:    465,
		Width:  830,
		Height: 235,
	}

	sg := &strokeGroup{
		width:  0.0,
		height: 0.0,
		strokes: []stroke{
			{
				points: []point{
					{
						x: 100.0,
						y: 110.5,
					},
					{
						x: 120.5,
						y: 130.0,
					},
				},
			},
		},
	}

	sg.scale(canvasLoc)

	if !reflect.DeepEqual(sg, want) {
		t.Errorf("scale() = %+v; want %+v", sg, want)
	}
}
