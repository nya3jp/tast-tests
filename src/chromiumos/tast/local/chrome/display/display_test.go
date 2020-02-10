// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package display

import (
	"reflect"
	"testing"
)

func TestSelectedMode(t *testing.T) {
	for _, c := range []struct {
		name         string
		info         *Info
		expectedMode *DisplayMode
	}{
		{
			"multi",
			&Info{
				Modes: []*DisplayMode{
					&DisplayMode{Width: 640, Height: 480, IsNative: true, IsSelected: false},
					&DisplayMode{Width: 320, Height: 240, DeviceScaleFactor: 2.0, IsNative: false, IsSelected: true},
				},
			},
			&DisplayMode{Width: 320, Height: 240, DeviceScaleFactor: 2.0, IsNative: false, IsSelected: true},
		},
		{
			"error-no-selected",
			&Info{
				Modes: []*DisplayMode{
					&DisplayMode{Width: 640, Height: 480, IsNative: true, IsSelected: false},
					&DisplayMode{Width: 320, Height: 240, DeviceScaleFactor: 2.0, IsNative: false, IsSelected: false},
				},
			},
			nil,
		},
		{
			"error-empty",
			&Info{},
			nil,
		},
		{
			"error-multiple-selected",
			&Info{
				Modes: []*DisplayMode{
					&DisplayMode{Width: 640, Height: 480, IsNative: true, IsSelected: true},
					&DisplayMode{Width: 320, Height: 240, DeviceScaleFactor: 2.0, IsNative: false, IsSelected: true},
				},
			},
			&DisplayMode{Width: 640, Height: 480, IsNative: true, IsSelected: true},
		},
		{
			"error-multiple-selected2",
			&Info{
				Modes: []*DisplayMode{
					&DisplayMode{Width: 320, Height: 240, DeviceScaleFactor: 2.0, IsNative: false, IsSelected: true},
					&DisplayMode{Width: 640, Height: 480, IsNative: true, IsSelected: true},
				},
			},
			&DisplayMode{Width: 320, Height: 240, DeviceScaleFactor: 2.0, IsNative: false, IsSelected: true},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			gotMode, err := c.info.GetSelectedMode()
			if c.expectedMode == nil {
				if err == nil {
					t.Errorf("Expected to fail, but successfully got %+v", gotMode)
				}
			} else {
				if err != nil {
					t.Error("Failed to get the selected info: ", err)
				}
				if !reflect.DeepEqual(c.expectedMode, gotMode) {
					t.Errorf("Unexpected mode is selected: want %+v, got %+v", c.expectedMode, gotMode)
				}
			}
		})
	}
}

func TestEffectiveDeviceScaleFactor(t *testing.T) {
	for _, c := range []struct {
		name          string
		info          *Info
		shouldSuccess bool
		expectedDSF   float64
	}{
		{
			"normal",
			&Info{
				DisplayZoomFactor: 1.0,
				Modes: []*DisplayMode{
					&DisplayMode{DeviceScaleFactor: 1.0, IsSelected: true},
				},
			},
			true, 1.0,
		},
		{
			"display-zoomed",
			&Info{
				DisplayZoomFactor: 1.2,
				Modes: []*DisplayMode{
					&DisplayMode{DeviceScaleFactor: 1.0, IsSelected: true},
				},
			},
			true, 1.2,
		},
		{
			"hidpi",
			&Info{
				DisplayZoomFactor: 1.0,
				Modes: []*DisplayMode{
					&DisplayMode{DeviceScaleFactor: 2.0, IsSelected: true},
				},
			},
			true, 2.0,
		},
		{
			"hidpi-zoomed",
			&Info{
				DisplayZoomFactor: 1.2,
				Modes: []*DisplayMode{
					&DisplayMode{DeviceScaleFactor: 2.0, IsSelected: true},
				},
			},
			true, 2.4,
		},
		{
			"error",
			&Info{
				DisplayZoomFactor: 1.0,
			},
			false, 1.0,
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			gotDSF, err := c.info.GetEffectiveDeviceScaleFactor()
			if c.shouldSuccess {
				if err != nil {
					t.Fatal("Failed to get DSF: ", err)
				}
				if c.expectedDSF != gotDSF {
					t.Errorf("Incorrect DSF: want %f got %f", c.expectedDSF, gotDSF)
				}
			} else {
				if err == nil {
					t.Errorf("Expected to fail, but successfully got DSF %f", gotDSF)
				}
			}
		})
	}
}
