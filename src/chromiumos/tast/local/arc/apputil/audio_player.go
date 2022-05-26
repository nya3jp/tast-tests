// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package apputil implements the libraries used to control ARC apps
package apputil

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
)

// ARCAudioPlayer specifies the features of an ARC++ app that can play an audio or music.
type ARCAudioPlayer interface {
	// Install installs the app.
	Install(ctx context.Context) error

	// Launch launches the app and returns the time spent for the app to be visible.
	Launch(ctx context.Context) (time.Duration, error)

	// Play plays the specified music/song via the app.
	Play(ctx context.Context, audio *Audio) error

	// Close closes the app.
	Close(ctx context.Context, cr *chrome.Chrome, hasError func() bool, outDir string) error
}

// Audio collects the information of an audio that ARCAudioPlayer is going to search and play.
type Audio struct {
	Query    string // Query is the query text of the audio.
	Subtitle string // Subtitle is the sbutitle of the audio.
}

// NewAudio returns the audio that ARCAudioPlayer is going to search and play
func NewAudio(query, subtitle string) *Audio {
	return &Audio{
		Query:    query,
		Subtitle: subtitle,
	}
}
