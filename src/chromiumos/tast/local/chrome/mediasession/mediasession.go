// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package mediasession contains common utilities to help writing media session tests.
package mediasession

import (
	"context"

	"chromiumos/tast/local/chrome"
)

// State represents the current state of the audio playing.
type State string

const (
	// StatePaused represents that the audio is paused.
	StatePaused State = "paused"

	// StatePlaying represents that the audio is playing.
	StatePlaying State = "playing"
)

// Conn is the connection to the test page.
type Conn struct {
	*chrome.Conn
}

// LoadTestPage opens the media session test page in Chrome. The caller is responsible for closing the returned Conn.
// Tests should symlink data/media_session_test.html into their own data directory and pass the URL
// at which is available via the url argument.
func LoadTestPage(ctx context.Context, cr *chrome.Chrome, url string) (*Conn, error) {
	conn, err := cr.NewConn(ctx, url)
	if err != nil {
		return nil, err
	}
	return &Conn{Conn: conn}, nil
}

// Play starts to play the audio and checks that it has successfully started playing.
func (c *Conn) Play(ctx context.Context) error {
	if err := c.Conn.Exec(ctx, "audio.play()"); err != nil {
		return err
	}

	return c.Conn.WaitForExpr(ctx, "audio.currentTime > 0")
}

// State returns the current audio state which is whether it is playing or paused.
func (c *Conn) State(ctx context.Context) (State, error) {
	var paused bool
	if err := c.Conn.Eval(ctx, "audio.paused", &paused); err != nil {
		return StatePaused, err
	}
	if paused {
		return StatePaused, nil
	}
	return StatePlaying, nil
}

// WaitForState waits for the audio state becoming the given one.
func (c *Conn) WaitForState(ctx context.Context, state State) error {
	expr := "audio.paused"
	if state == StatePlaying {
		expr = "!" + expr
	}
	return c.Conn.WaitForExpr(ctx, expr)
}
