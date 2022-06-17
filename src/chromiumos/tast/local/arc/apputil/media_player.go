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

// ARCMediaPlayer specifies the features of an ARC++ app that can play an audio or video.
type ARCMediaPlayer interface {
	// Install installs the app.
	Install(ctx context.Context) error

	// Launch launches the app and returns the time spent for the app to be visible.
	Launch(ctx context.Context) (time.Duration, error)

	// Play plays the specified music/song/video via the app.
	Play(ctx context.Context, media *Media) error

	// Close closes the app.
	Close(ctx context.Context, cr *chrome.Chrome, hasError func() bool, outDir string) error
}

// Media collects the information of an media that ARCMediaPlayer is going to search and play.
type Media struct {
	Query    string // Query is the query text of the media.
	Subtitle string // Subtitle is the sbutitle of the media.
}

// NewMedia returns the media that ARCMediaPlayer is going to search and play
func NewMedia(query, subtitle string) *Media {
	return &Media{
		Query:    query,
		Subtitle: subtitle,
	}
}
