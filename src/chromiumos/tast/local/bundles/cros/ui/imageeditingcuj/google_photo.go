// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package imageeditingcuj contains imageedit CUJ test cases library.
package imageeditingcuj

import (
	"context"

	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/input"
)

// GooglePhotos holds the information used to do Google Photos testing.
type GooglePhotos struct {
	cr       *chrome.Chrome
	tconn    *chrome.TestConn
	ui       *uiauto.Context
	uiHdl    cuj.UIActionHandler
	kb       *input.KeyboardEventWriter
	conn     *chrome.Conn
	password string
}

// NewGooglePhotos returns the the manager of Google Photos, caller will able to control Google Photos through this object.
func NewGooglePhotos(cr *chrome.Chrome, tconn *chrome.TestConn, uiHdl cuj.UIActionHandler, kb *input.KeyboardEventWriter, password string) *GooglePhotos {
	return &GooglePhotos{
		cr:       cr,
		tconn:    tconn,
		ui:       uiauto.New(tconn),
		uiHdl:    uiHdl,
		kb:       kb,
		password: password,
	}
}

// Open opens the Google Photos.
func (g *GooglePhotos) Open() uiauto.Action {
	return func(ctx context.Context) (err error) {
		return nil
	}
}

// Upload uploads the photo from the Downloads directory.
func (g *GooglePhotos) Upload(fileName string) uiauto.Action {
	return nil
}

// AddFilters adds filters to the photo.
func (g *GooglePhotos) AddFilters() uiauto.Action {
	return nil
}

// Edit changes brightness, sharpness and color depth.
func (g *GooglePhotos) Edit() uiauto.Action {
	return nil
}

// Rotate rotates the photo.
func (g *GooglePhotos) Rotate() uiauto.Action {
	return nil
}

// ReduceColor reduces colors to convert photos to black and white.
func (g *GooglePhotos) ReduceColor() uiauto.Action {
	return nil
}

// Crop crops the photo.
func (g *GooglePhotos) Crop() uiauto.Action {
	return nil
}

// UndoEdit undo all edits.
func (g *GooglePhotos) UndoEdit() uiauto.Action {
	return nil
}

// CleanUp deletes the photo uploaded at the beginning of the test case.
func (g *GooglePhotos) CleanUp() uiauto.Action {
	return nil
}

// Close closes the connection.
func (g *GooglePhotos) Close(ctx context.Context) {
}
