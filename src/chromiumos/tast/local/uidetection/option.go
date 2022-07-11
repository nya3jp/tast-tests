// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package uidetection

import "time"

// Options provides all of the ways which you can configure the UI detection API.
type Options struct {
	// The number of times of retries.
	Retries int
	// The interval between retries.
	RetryInterval time.Duration
	// Move the mouse to the right center of the screen after clicking, so that
	// it does not interfere with screenshots when the application renders the
	// cursor.
	MoveAfterClick bool
}

// DefaultOptions return options with default values.
func DefaultOptions() Options {
	return Options{
		Retries:        1,
		RetryInterval:  time.Second,
		MoveAfterClick: false,
	}
}

// Option is a modifier to apply to Options.
type Option = func(*Options)

// Retries controls the number of retries.
func Retries(retries int) Option {
	return func(o *Options) { o.Retries = retries }
}

// RetryInterval controls the interval between retries.
func RetryInterval(retryInterval time.Duration) Option {
	return func(o *Options) { o.RetryInterval = retryInterval }
}

// MoveAfterClick enables moving the cursor out of the way after every click.
func MoveAfterClick() Option {
	return func(o *Options) {
		o.MoveAfterClick = true
	}
}
