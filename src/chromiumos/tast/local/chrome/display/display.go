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
	"errors"
	"fmt"

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

// Insets holds oscreen insets.
// See https://developer.chrome.com/apps/system.display#type-Insets.
type Insets struct {
	Left   int `json:"left"`
	Top    int `json:"top"`
	Right  int `json:"right"`
	Bottom int `json:"bottom"`
}

// DisplayMode holds a mode supported by the display.
// See https://developer.chrome.com/apps/system.display#type-DisplayMode.
type DisplayMode struct {
	Width                int     `json:"width"`
	Height               int     `json:"height"`
	WidthInNativePixels  int     `json:"widthInNativePixels"`
	HeightInNativePixels int     `json:"heightInNativePixels"`
	UIScale              float64 `json:"uiScale"`
	DeviceScaleFactor    float64 `json:"deviceScaleFactor"`
	IsNative             bool    `json:"isNative"`
	IsSelected           bool    `json:"isSelected"`
}

// Info holds information about a display and is returned by GetInfo.
// See https://developer.chrome.com/apps/system_display#method-getInfo.
type Info struct {
	ID                string         `json:"id"`
	Name              string         `json:"name"`
	MirroringSourceID string         `json:"mirroringSourceId"`
	IsPrimary         bool           `json:"isPrimary"`
	IsInternal        bool           `json:"isInternal"`
	IsEnabled         bool           `json:"isEnabled"`
	IsUnified         bool           `json:"isUnified"`
	Bounds            *Bounds        `json:"bounds"`
	Overscan          *Insets        `json:"overscan"`
	WorkArea          *Bounds        `json:"workArea"`
	Modes             []*DisplayMode `json:"modes"`
	DPIX              float64        `json:"dpiX"`
	DPIY              float64        `json:"dpiY"`
	Rotation          int            `json:"rotation"`
	HasTouchSupport   bool           `json:"hasTouchSupport"`
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
		return nil, fmt.Errorf("failed to get display info: ", err)
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
type DisplayProperties struct {
	IsUnified         *bool        `json:"isUnified,omitempty"`
	MirroringSourceID *string      `json:"mirroringSourceId,omitempty"`
	IsPrimary         *bool        `json:"isPrimary,omitempty"`
	Overscan          *Insets      `json:"overscan,omitempty"`
	Rotation          *int         `json:"rotation,omitempty"`
	BoundsOriginX     *int         `json:"boundsOriginX,omitempty"`
	BoundsOriginY     *int         `json:"boundsOriginY,omitempty"`
	DisplayMode       *DisplayMode `json:"displayMode,omitempty"`
}

// SetDisplayProperties updates the properties for the display specified by id.
// See https://developer.chrome.com/apps/system_display#method-setDisplayProperties.
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
