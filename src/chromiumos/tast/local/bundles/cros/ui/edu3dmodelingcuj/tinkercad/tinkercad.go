// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package tinkercad implements TinkerCAD web operations.
package tinkercad

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/pointer"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/uidetection"
)

// TinkerCad defines the struct related to TinkerCAD website.
type TinkerCad struct {
	ui             *uiauto.Context
	ud             *uidetection.Context
	conn           *chrome.Conn
	tconn          *chrome.TestConn
	kb             *input.KeyboardEventWriter
	pc             *pointer.MouseContext
	EditorWinRect  coords.Rect
	exportFilePath string
	rotateIconPath string
	addShapeCount  int
}

// ViewMode defines several view mode's node finder.
type ViewMode *nodewith.Finder

// NewTinkerCad creates an instance of TinkerCAD.
func NewTinkerCad(tconn *chrome.TestConn, kb *input.KeyboardEventWriter, rotateIconPath string) *TinkerCad {
	return &TinkerCad{
		ui:             uiauto.New(tconn),
		ud:             uidetection.NewDefault(tconn),
		tconn:          tconn,
		kb:             kb,
		pc:             pointer.NewMouse(tconn),
		EditorWinRect:  coords.Rect{},
		rotateIconPath: rotateIconPath,
	}
}

// Open opens a TinkerCAD website on chrome browser.
// TODO(b/236668705): Implement the function.
func (tc *TinkerCad) Open(ctx context.Context, cr *chrome.Chrome) {}

// Login logs in to TinkerCAD with google oauth.
// Will do sign up process if don't have a TinkerCAD account.
// TODO(b/236668705): Implement the function.
func (tc *TinkerCad) Login(ctx context.Context, account string) {}

// Copy copies an initial design from sample design URL and returns the name of the design.
// TODO(b/236668705): Implement the function.
func (tc *TinkerCad) Copy(ctx context.Context, sampleDesignURL string) {}

// Delete deletes the specific design by it's name.
// TODO(b/236668705): Implement the function.
func (tc *TinkerCad) Delete(ctx context.Context, sampleDesignName string) {}

// GetEditorWindowRect gets editor window's rectangular region.
// The editor window is the design editing area of TinkerCAD.
// TODO(b/236668705): Implement the function.
func (tc *TinkerCad) GetEditorWindowRect(ctx context.Context) {}

// DisableGrid disable show grid option.
// TODO(b/236668705): Implement the function.
func (tc *TinkerCad) DisableGrid() {}

// AddShapeAndRotate adds primitive shape by given shape name and rotates it.
// TODO(b/236668705): Implement the function.
func (tc *TinkerCad) AddShapeAndRotate(shapeName string) {}

// RotateAll rotates all shapes at the same time.
// TODO(b/236668705): Implement the function.
func (tc *TinkerCad) RotateAll() {}

// RotateViewCube rotates the design with viewcube.
// TODO(b/236668705): Implement the function.
func (tc *TinkerCad) RotateViewCube() {}

// Visualize visualizes the design in given view mode.
// TODO(b/236668705): Implement the function.
func (tc *TinkerCad) Visualize(view ViewMode) {}

// ExportAndVerify exports the design and verifies successfully exported.
// TODO(b/236668705): Implement the function.
func (tc *TinkerCad) ExportAndVerify(ctx context.Context, downloadsPath, sampleDesignName string) {}

// Close closes the TinkerCAD website.
// TODO(b/236668705): Implement the function.
func (tc *TinkerCad) Close(ctx context.Context) {}

// getShapePoint gets primitive shape point with query selector.
// TODO(b/236668705): Implement the function.
func (tc *TinkerCad) getShapePoint(ctx context.Context, shapeSelector string) {}

// rotate gets location by uidetection and rotates the icon.
// TODO(b/236668705): Implement the function.
func (tc *TinkerCad) rotate(ctx context.Context) {}
