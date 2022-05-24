// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package display

import (
	"testing"
)

func TestSelectedMode(t *testing.T) {
	for _, c := range []struct {
		name string
		info *Info
		// The index in 'Modes' to be returned, or -1 if error is expected.
		expectedIndex int
	}{
		{
			"multi",
			&Info{
				Modes: []*DisplayMode{
					{Width: 640, Height: 480, IsNative: true, IsSelected: false},
					{Width: 320, Height: 240, DeviceScaleFactor: 2.0, IsNative: false, IsSelected: true},
				},
			},
			1,
		},
		{
			"error-no-selected",
			&Info{
				Modes: []*DisplayMode{
					{Width: 640, Height: 480, IsNative: true, IsSelected: false},
					{Width: 320, Height: 240, DeviceScaleFactor: 2.0, IsNative: false, IsSelected: false},
				},
			},
			-1,
		},
		{
			"error-empty",
			&Info{},
			-1,
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			gotMode, err := c.info.GetSelectedMode()
			if c.expectedIndex < 0 {
				if err == nil {
					t.Errorf("Expected to fail, but successfully got %+v", gotMode)
				}
			} else {
				if err != nil {
					t.Error("Failed to get the selected info: ", err)
				}
				if gotMode != c.info.Modes[c.expectedIndex] {
					t.Errorf("Unexpected mode is selected: got %+v, want %+v", gotMode, c.info.Modes[c.expectedIndex])
				}
			}
		})
	}
}

func TestEffectiveDeviceScaleFactor(t *testing.T) {
	for _, c := range []struct {
		name          string
		info          *Info
		shouldSucceed bool
		expectedDSF   float64
	}{
		{
			"normal",
			&Info{
				DisplayZoomFactor: 1.0,
				Modes: []*DisplayMode{
					{DeviceScaleFactor: 1.0, IsSelected: true},
				},
			},
			true, 1.0,
		},
		{
			"display-zoomed",
			&Info{
				DisplayZoomFactor: 1.2,
				Modes: []*DisplayMode{
					{DeviceScaleFactor: 1.0, IsSelected: true},
				},
			},
			true, 1.2,
		},
		{
			"hidpi",
			&Info{
				DisplayZoomFactor: 1.0,
				Modes: []*DisplayMode{
					{DeviceScaleFactor: 2.0, IsSelected: true},
				},
			},
			true, 2.0,
		},
		{
			"hidpi-zoomed",
			&Info{
				DisplayZoomFactor: 1.2,
				Modes: []*DisplayMode{
					{DeviceScaleFactor: 2.0, IsSelected: true},
				},
			},
			true, 2.4,
		},
		{
			"error-empty",
			&Info{
				DisplayZoomFactor: 1.0,
			},
			false, 1.0,
		},
		{
			"error-not-selected",
			&Info{
				DisplayZoomFactor: 1.0,
				Modes: []*DisplayMode{
					{DeviceScaleFactor: 2.0, IsSelected: false},
				},
			},
			false, 1.0,
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			gotDSF, err := c.info.GetEffectiveDeviceScaleFactor()
			if c.shouldSucceed {
				if err != nil {
					t.Fatal("Failed to get DSF: ", err)
				}
				if gotDSF != c.expectedDSF {
					t.Errorf("Incorrect DSF: got %f, want %f", gotDSF, c.expectedDSF)
				}
			} else {
				if err == nil {
					t.Errorf("Expected to fail, but successfully got DSF %f", gotDSF)
				}
			}
		})
	}
}
