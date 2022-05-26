// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package apputil implements the libraries used to control ARC apps
package apputil

import (
	"context"

	"chromiumos/tast/local/chrome"
)

// ARCAudioPlayer specifies the features of an ARC++ app that can pley an audio or music.
type ARCAudioPlayer interface {
	// Install installs the app.
	Install(ctx context.Context) error

	// Launch launches the app.
	Launch(ctx context.Context) error

	// Play plays the specified music/song via the app.
	Play(ctx context.Context, audio *Audio) error

	// Close closes the app.
	Close(ctx context.Context, cr *chrome.Chrome, hasError func() bool, outDir string) error
}

// Audio collects the information of an audio that ARCAudioPlayer is going to search and play.
// not all items need to be specified, the way to search and play an audio is highly dependent on different apps.
// The app should provide a function that returns an instance of this struct.
type Audio struct {
	Query    string // Query is the query text of the audio.
	SubTitle string // Title is the sbutitle of the audio.
	Album    string // Album is the album of the audio.
	Artist   string // Title is the artist of the audio.
}
