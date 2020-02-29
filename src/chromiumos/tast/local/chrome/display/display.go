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
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
)

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
	Bounds                      coords.Rect    `json:"bounds"`
	Overscan                    *Insets        `json:"overscan"`
	WorkArea                    coords.Rect    `json:"workArea"`
	Modes                       []*DisplayMode `json:"modes"`
	HasTouchSupport             bool           `json:"hasTouchSupport"`
	AvailableDisplayZoomFactors []float64      `json:"availableDisplayZoomFactors"`
	DisplayZoomFactor           float64        `json:"displayZoomFactor"`
}

// GetSelectedMode returns the currently selected display mode. It returns
// nil if no such mode is found.
func (info *Info) GetSelectedMode() (*DisplayMode, error) {
	for _, mode := range info.Modes {
		if mode.IsSelected {
			return mode, nil
		}
	}
	return nil, errors.New("no modes are selected")
}

// GetEffectiveDeviceScaleFactor computes the ratio of a DIP (device independent
// pixel) to a physical pixel, which is DisplayZoomFactor x DeviceScaleFactor.
// See also ui/display/manager/managed_display_info.h in Chromium.
func (info *Info) GetEffectiveDeviceScaleFactor() (float64, error) {
	mode, err := info.GetSelectedMode()
	if err != nil {
		return 0, err
	}
	return info.DisplayZoomFactor * mode.DeviceScaleFactor, nil
}

// GetInfo calls chrome.system.display.getInfo to get information about connected displays.
// See https://developer.chrome.com/apps/system_display#method-getInfo.
func GetInfo(ctx context.Context, tconn *chrome.TestConn) ([]Info, error) {
	var infos []Info
	if err := tconn.EvalPromise(ctx, `tast.promisify(chrome.system.display.getInfo)()`, &infos); err != nil {
		return nil, errors.Wrap(err, "failed to get display info")
	}
	if len(infos) == 0 {
		// At leasat one display info should exist. So empty info would mean
		// something is wrong.
		return nil, errors.New("no display info are contained")
	}
	return infos, nil
}

// GetInternalInfo returns information about the internal display.
// An error is returned if no internal display is present.
func GetInternalInfo(ctx context.Context, tconn *chrome.TestConn) (*Info, error) {
	infos, err := GetInfo(ctx, tconn)
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
func SetDisplayProperties(ctx context.Context, tconn *chrome.TestConn, id string, dp DisplayProperties) error {
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
	if err = tconn.EvalPromise(ctx, expr, &msg); err != nil {
		return err
	} else if msg != "" {
		return errors.New(msg)
	}
	return nil
}

// RotationAngle represents the supported rotation angles by SetDisplayRotationSync.
type RotationAngle string

// Rotation values as defined in: https://cs.chromium.org/chromium/src/out/Debug/gen/chrome/common/extensions/api/autotest_private.h
const (
	// Rotate0 represents rotation angle 0.
	Rotate0 RotationAngle = "Rotate0"
	// Rotate90 represents rotation angle 90.
	Rotate90 RotationAngle = "Rotate90"
	// Rotate180 represents rotation angle 180.
	Rotate180 RotationAngle = "Rotate180"
	// Rotate270 represents rotation angle 270.
	Rotate270 RotationAngle = "Rotate270"
)

// SetDisplayRotationSync rotates the display to a certain angle and waits until the rotation animation finished.
// c must be a connection with both system.display and autotestPrivate permissions.
func SetDisplayRotationSync(ctx context.Context, tconn *chrome.TestConn, dispID string, rot RotationAngle) error {
	var rotInt int
	switch rot {
	case Rotate0:
		rotInt = 0
	case Rotate90:
		rotInt = 90
	case Rotate180:
		rotInt = 180
	case Rotate270:
		rotInt = 270
	default:
		return errors.Errorf("unexpected rotation value; got %q, want: any of [Rotate0,Rotate90,Rotate180,Rotate270]", rot)
	}

	p := DisplayProperties{Rotation: &rotInt}
	if err := SetDisplayProperties(ctx, tconn, dispID, p); err != nil {
		return errors.Wrapf(err, "failed to set rotation to %d", rotInt)
	}

	expr := fmt.Sprintf(
		`tast.promisify(chrome.autotestPrivate.waitForDisplayRotation)(%q, %q).then((success) => {
		    if (!success)
		      throw new Error("failed to wait for display rotation");
		})`, dispID, rot)
	return tconn.EvalPromise(ctx, expr, nil)
}

// OrientationType represents a display orientation.
type OrientationType string

// OrientationType values as "enum OrientationType" defined in https://w3c.github.io/screen-orientation/#screenorientation-interface
const (
	OrientationPortraitPrimary    OrientationType = "portrait-primary"
	OrientationPortraitSecondary  OrientationType = "portrait-secondary"
	OrientationLandscapePrimary   OrientationType = "landscape-primary"
	OrientationLandscapeSecondary OrientationType = "landscape-secondary"
)

// Orientation holds information obtained from the screen orientation API.
// See https://w3c.github.io/screen-orientation/#screenorientation-interface
type Orientation struct {
	// Angle is an angle in degrees of the display counterclockwise from the
	// orientation of the display panel.
	Angle int `json:"angle"`
	// Type is an OrientationType representing the display orientation.
	Type OrientationType `json:"type"`
}

// GetOrientation returns the Orientation of the display.
func GetOrientation(ctx context.Context, tconn *chrome.TestConn) (*Orientation, error) {
	result := &Orientation{}
	// Using a JS expression to evaluate screen.orientation to a JSON object
	// because JSON.stringify does not work for it and returns {}.
	if err := tconn.Eval(ctx, `s=screen.orientation;o={"angle":s.angle,"type":s.type}`, result); err != nil {
		return nil, err
	}
	return result, nil
}

// IsFakeDisplayID checks if a display is fake or not by its id.
func IsFakeDisplayID(id string) bool {
	// the id of fake displays will start from this number.
	// See also: https://source.chromium.org/chromium/chromium/src/+/master:ui/display/manager/managed_display_info.cc?q=%20kSynthesizedDisplayIdStart
	const fakeDisplayID = "2200000000"

	// Theoretically it is possible that a fake display has a different ID. This
	// happens when some displays are connected and then disconnected; this is
	// unlikely to happen on test environment.
	return id == fakeDisplayID
}

// PhysicalDisplayConnected checks the display info and returns true if at least
// one physical display is connected.
func PhysicalDisplayConnected(ctx context.Context, tconn *chrome.TestConn) (bool, error) {
	infos, err := GetInfo(ctx, tconn)
	if err != nil {
		return false, err
	}
	if len(infos) > 1 {
		return true, nil
	}
	return !IsFakeDisplayID(infos[0].ID), nil
}

// NewTouchCoordConverter creates a new TouchCoordConverter for the internal
// display with the given TouchscreenEventWriter.
func NewTouchCoordConverter(ctx context.Context, tconn *chrome.TestConn, tsew *input.TouchscreenEventWriter) (*coords.TouchCoordConverter, error) {
	info, err := GetInternalInfo(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "no internal display found")
	}

	return coords.NewTouchCoordConverter(info.Bounds.Size(), tsew), nil
}
