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
// TODO(b/233682664): Implement the function.
func (g *GooglePhotos) Open() {}

// Upload uploads the photo from the Downloads directory.
// TODO(b/233682664): Implement the function.
func (g *GooglePhotos) Upload(fileName string) {}

// AddFilters adds filters to the photo.
// TODO(b/233682664): Implement the function.
func (g *GooglePhotos) AddFilters() {}

// Edit changes brightness, sharpness and color depth.
// TODO(b/233682664): Implement the function.
func (g *GooglePhotos) Edit() {}

// Rotate rotates the photo.
// TODO(b/233682664): Implement the function.
func (g *GooglePhotos) Rotate() {}

// ReduceColor reduces colors to convert photos to black and white.
// TODO(b/233682664): Implement the function.
func (g *GooglePhotos) ReduceColor() {}

// Crop crops the photo.
// TODO(b/233682664): Implement the function.
func (g *GooglePhotos) Crop() {}

// UndoEdit undo all edits.
// TODO(b/233682664): Implement the function.
func (g *GooglePhotos) UndoEdit() {}

// CleanUp deletes the photo uploaded at the beginning of the test case.
// TODO(b/233682664): Implement the function.
func (g *GooglePhotos) CleanUp() {}

// Close closes the connection.
// TODO(b/233682664): Implement the function.
func (g *GooglePhotos) Close(ctx context.Context) {}
