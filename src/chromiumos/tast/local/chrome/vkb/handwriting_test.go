// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vkb

import (
	"reflect"
	"strings"
	"testing"

	"chromiumos/tast/local/coords"
)

func TestNewStrokeGroup(t *testing.T) {
	want := &strokeGroup{
		width:  100.0,
		height: 200.0,
		strokes: []stroke{
			stroke{
				points: []point{
					point{
						x: 5.0,
						y: 10.5,
					},
					point{
						x: 6.0,
						y: 11.7,
					},
				},
			},
			stroke{
				points: []point{
					point{
						x: 2.0,
						y: 3.4,
					},
				},
			},
		},
	}

	got, _ := newStrokeGroup(strings.NewReader("100.0 200.0\n5.0 10.5 6.0 11.7\n 2.0 3.4"))

	if !reflect.DeepEqual(got, want) {
		t.Errorf("newStrokeGroup() = %+v, want %+v", got, want)
	}
}

func TestScaleHandwritingData(t *testing.T) {
	want := &strokeGroup{
		width:  105.75,
		height: 141.0,
		strokes: []stroke{
			stroke{
				points: []point{
					point{
						x: 811.625,
						y: 1252.25,
					},
				},
			},
		},
	}

	mockCanvasLoc := coords.Rect{
		Left:   97,
		Top:    465,
		Width:  830,
		Height: 235,
	}

	mockStrokeContainer := &strokeGroup{
		width:  1.5,
		height: 2.0,
		strokes: []stroke{
			stroke{
				points: []point{
					point{
						x: 5.0,
						y: 10.5,
					},
				},
			},
		},
	}

	scaleHandwritingData(mockStrokeContainer, mockCanvasLoc)

	if !reflect.DeepEqual(mockStrokeContainer, want) {
		t.Errorf("scaleHandwritingData() = %+v, want %+v", mockStrokeContainer, want)
	}
}
