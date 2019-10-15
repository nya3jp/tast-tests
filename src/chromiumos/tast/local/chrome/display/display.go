// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package display wraps the chrome.system.display API.
//
// Functions require a chrome.Conn with permission to use the chrome.system.display API.
// chrome.Chrome.TestAPIConn has such permission and may be passed here.
package display

import (
	"context"
	"encoding/json"
	"fmt"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
)

// Bounds holds onscreen bounds.
// See https://developer.chrome.com/apps/system.display#type-Bounds.
type Bounds struct {
	Left   int `json:"left"`
	Top    int `json:"top"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// Insets holds onscreen insets.
// See https://developer.chrome.com/apps/system.display#type-Insets.
type Insets struct {
	Left   int `json:"left"`
	Top    int `json:"top"`
	Right  int `json:"right"`
	Bottom int `json:"bottom"`
}

// DisplayMode holds a mode supported by the display.
// See https://developer.chrome.com/apps/system.display#type-DisplayMode.
type DisplayMode struct { // NOLINT
	Width                int     `json:"width"`
	Height               int     `json:"height"`
	WidthInNativePixels  int     `json:"widthInNativePixels"`
	HeightInNativePixels int     `json:"heightInNativePixels"`
	UIScale              float64 `json:"uiScale,omitempty"`
	DeviceScaleFactor    float64 `json:"deviceScaleFactor"`
	IsNative             bool    `json:"isNative"`
	IsSelected           bool    `json:"isSelected"`
	IsInterlaced         bool    `json:"isInterlaced,omitempty"`
}

// Info holds information about a display and is returned by GetInfo.
// See https://developer.chrome.com/apps/system_display#method-getInfo.
type Info struct {
	ID                          string         `json:"id"`
	Name                        string         `json:"name"`
	MirroringSourceID           string         `json:"mirroringSourceId"`
	IsPrimary                   bool           `json:"isPrimary"`
	IsInternal                  bool           `json:"isInternal"`
	IsEnabled                   bool           `json:"isEnabled"`
	IsUnified                   bool           `json:"isUnified"`
	DPIX                        float64        `json:"dpiX"`
	DPIY                        float64        `json:"dpiY"`
	Rotation                    int            `json:"rotation"`
	Bounds                      *Bounds        `json:"bounds"`
	Overscan                    *Insets        `json:"overscan"`
	WorkArea                    *Bounds        `json:"workArea"`
	Modes                       []*DisplayMode `json:"modes"`
	HasTouchSupport             bool           `json:"hasTouchSupport"`
	AvailableDisplayZoomFactors []float64      `json:"availableDisplayZoomFactors"`
	DisplayZoomFactor           float64        `json:"displayZoomFactor"`
}

// GetInfo calls chrome.system.display.getInfo to get information about connected displays.
// See https://developer.chrome.com/apps/system_display#method-getInfo.
func GetInfo(ctx context.Context, c *chrome.Conn) ([]Info, error) {
	infos := make([]Info, 0)
	err := c.EvalPromise(ctx,
		`new Promise(function(resolve, reject) {
		  chrome.system.display.getInfo(function(info) { resolve(info); });
		})`, &infos)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get display info")
	}
	return infos, nil
}

// GetInternalInfo returns information about the internal display.
// An error is returned if no internal display is present.
func GetInternalInfo(ctx context.Context, c *chrome.Conn) (*Info, error) {
	infos, err := GetInfo(ctx, c)
	if err != nil {
		return nil, err
	}
	for i := range infos {
		if infos[i].IsInternal {
			return &infos[i], nil
		}
	}
	return nil, errors.New("no internal display")
}

// DisplayProperties holds properties to change and is passed to SetDisplayProperties.
// nil fields are ignored. See https://developer.chrome.com/apps/system_display#method-setDisplayProperties.
type DisplayProperties struct { // NOLINT
	IsUnified         *bool        `json:"isUnified,omitempty"`
	MirroringSourceID *string      `json:"mirroringSourceId,omitempty"`
	IsPrimary         *bool        `json:"isPrimary,omitempty"`
	Overscan          *Insets      `json:"overscan,omitempty"`
	Rotation          *int         `json:"rotation,omitempty"`
	BoundsOriginX     *int         `json:"boundsOriginX,omitempty"`
	BoundsOriginY     *int         `json:"boundsOriginY,omitempty"`
	DisplayMode       *DisplayMode `json:"displayMode,omitempty"`
	DisplayZoomFactor *float64     `json:"displayZoomFactor,omitempty"`
}

// SetDisplayProperties updates the properties for the display specified by id.
// See https://developer.chrome.com/apps/system_display#method-setDisplayProperties.
// Some properties, like rotation, will be performed in an async way. For rotation in particular,
// you should call display.WaitForDisplayRotation() to know when the rotation animation finishes.
func SetDisplayProperties(ctx context.Context, c *chrome.Conn, id string, dp DisplayProperties) error {
	b, err := json.Marshal(&dp)
	if err != nil {
		return err
	}
	expr := fmt.Sprintf(
		`new Promise(function(resolve, reject) {
		  chrome.system.display.setDisplayProperties(
		      %q, %s, function() {
		    resolve(chrome.runtime.lastError ? chrome.runtime.lastError.message : "");
		  });
		})`, id, string(b))

	msg := ""
	if err = c.EvalPromise(ctx, expr, &msg); err != nil {
		return err
	} else if msg != "" {
		return errors.New(msg)
	}
	return nil
}

// WaitForDisplayRotation waits until a given display ID finishes the rotation animation.
// rot could be any value of 0, 90, 180 any 270.
// c must be a connection with autotestPrivate permissions.
func WaitForDisplayRotation(ctx context.Context, c *chrome.Conn, dispID string, rot int) error {
	// This functions usually will get called immediately after calling display.SetDisplayProperties(... rot),
	// where "rot" is an integer. To avoid conversion code in the caller function, we do it here.
	// Rotation values as defined in: https://cs.chromium.org/chromium/src/out/Debug/gen/chrome/common/extensions/api/autotest_private.h
	var rotStr string
	switch rot {
	case 0:
		rotStr = "Rotate0"
	case 90:
		rotStr = "Rotate90"
	case 180:
		rotStr = "Rotate180"
	case 270:
		rotStr = "Rotate270"
	default:
		return errors.Errorf("unexpected rotation value; got %d, want: any of [0,90,180,270]", rot)
	}
	expr := fmt.Sprintf(
		`new Promise(function(resolve, reject) {
		  chrome.autotestPrivate.waitForDisplayRotation(%q, %q, function(success) {
		    if (chrome.runtime.lastError) {
		      reject(new Error(chrome.runtime.lastError.message));
		      return;
		    }
		    if (!success) {
		      reject(new Error("failed to wait for display rotation"));
		      return;
		    }
		    resolve();
		  });
		})`, dispID, rotStr)
	return c.EvalPromise(ctx, expr, nil)
}
