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
		width:  100.0,
		height: 200.0,
		strokes: []stroke{
			{
				points: []point{
					{
						x: 5.0,
						y: 10.5,
					},
					point{
						x: 6.0,
						y: 11.7,
					},
				},
			},
			{
				points: []point{
					{
						x: 2.0,
						y: 3.4,
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

	if _, err := file.Write([]byte("100.0 200.0\n5.0 10.5 6.0 11.7\n 2.0 3.4")); err != nil {
		t.Fatal("Write() failed: ", err)
	}

	got, err := newStrokeGroup(file.Name())
	if err != nil {
		t.Fatal("newStrokeGroup() failed: ", err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("newStrokeGroup() = %+v; want %+v", got, want)
	}
}

func TestScale(t *testing.T) {
	want := &strokeGroup{
		width:  105.75,
		height: 141.0,
		strokes: []stroke{
			{
				points: []point{
					{
						x: 811.625,
						y: 1252.25,
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
		width:  1.5,
		height: 2.0,
		strokes: []stroke{
			{
				points: []point{
					{
						x: 5.0,
						y: 10.5,
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
