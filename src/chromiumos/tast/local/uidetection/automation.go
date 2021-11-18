// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package uidetection

import (
	"context"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/testing"
)

// Context provides functionalities for image-based UI automation.
type Context struct {
	tconn    *chrome.TestConn
	detector *uiDetector
	pollOpts testing.PollOptions
}

// New returns a new UI Detection automation instance.
func New(t *chrome.TestConn, keyType, key, server string) *Context {
	return &Context{
		tconn: t,
		detector: &uiDetector{
			keyType: keyType,
			key:     key,
			server:  server,
		},
		pollOpts: testing.PollOptions{
			Interval: 300 * time.Millisecond,
			Timeout:  15 * time.Second,
		},
	}
}

// WithTimeout returns a new Context with the specified timeout.
func (uda *Context) WithTimeout(timeout time.Duration) *Context {
	return &Context{
		tconn:    uda.tconn,
		detector: uda.detector,
		pollOpts: testing.PollOptions{
			Interval: uda.pollOpts.Interval,
			Timeout:  timeout,
		},
	}
}

// WithInterval returns a new Context with the specified polling interval.
func (uda *Context) WithInterval(interval time.Duration) *Context {
	return &Context{
		tconn:    uda.tconn,
		detector: uda.detector,
		pollOpts: testing.PollOptions{
			Interval: interval,
			Timeout:  uda.pollOpts.Timeout,
		},
	}
}

// WithPollOpts returns a new Context with the specified polling options.
func (uda *Context) WithPollOpts(pollOpts testing.PollOptions) *Context {
	return &Context{
		tconn:    uda.tconn,
		detector: uda.detector,
		pollOpts: pollOpts,
	}
}

func (uda *Context) click(s *Finder, button mouse.Button, optionList ...Option) uiauto.Action {
	// TODO(b/205235148): Consolidate uiauto for UI tree based finder and image based finder.
	options := DefaultOptions()
	for _, opt := range optionList {
		opt(options)
	}
	return action.Retry(options.Retries, func(ctx context.Context) error {
		loc, err := uda.Location(ctx, s)
		if err != nil {
			return errors.Wrapf(err, "failed to find the location of %q", s.desc)
		}
		return mouse.Click(uda.tconn, loc.CenterPoint(), button)(ctx)
	}, options.RetryInterval)
}

// LeftClick returns an action that left-clicks a finder.
func (uda *Context) LeftClick(s *Finder, optionList ...Option) uiauto.Action {
	return uda.click(s, mouse.LeftButton, optionList...)
}

// RightClick returns an action that right-clicks a finder.
func (uda *Context) RightClick(s *Finder, optionList ...Option) uiauto.Action {
	return uda.click(s, mouse.RightButton, optionList...)
}

// Location finds the location of a finder in the screen.
func (uda *Context) Location(ctx context.Context, s *Finder) (*Location, error) {
	if err := s.resolve(ctx, uda.detector, uda.pollOpts); err != nil {
		return nil, errors.Wrapf(err, "failed to resolve the finder: %q", s.desc)
	}
	return s.location()
}

// WaitUntilExists keeps polling for a specified element to show up.
func (uda *Context) WaitUntilExists(s *Finder) uiauto.Action {
	options := DefaultOptions()
	options.Retries = 15
	options.RetryInterval = 1 * time.Second

	return action.Retry(options.Retries, func(ctx context.Context) error {
		_, err := uda.Location(ctx, s)
		if err != nil {
			return errors.Wrapf(err, "failed to find the location of %q", s.desc)
		}

		return nil
	}, options.RetryInterval)
}
