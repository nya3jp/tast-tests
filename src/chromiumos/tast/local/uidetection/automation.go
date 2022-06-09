// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package uidetection

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/touch"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

var serverKeyType = testing.RegisterVarString(
	KeyType,
	"",
	"The key type of ChromeOS UI detection server",
)
var serverKey = testing.RegisterVarString(
	Key,
	"",
	"The key of ChromeOS UI detection server",
)
var serverKeyAddr = testing.RegisterVarString(
	Server,
	"",
	"The address of ChromeOS UI detection server",
)

// ScreenshotStrategy holds the different screenshot strategies that can be used for image-based UI detection.
type ScreenshotStrategy int

// Holds all the screenshot types that can be used.
const (
	StableScreenshot ScreenshotStrategy = iota
	ImmediateScreenshot
)

// Context provides functionalities for image-based UI automation.
type Context struct {
	tconn              *chrome.TestConn
	detector           *uiDetector
	pollOpts           testing.PollOptions
	options            *Options
	screenshotStrategy ScreenshotStrategy
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
			Timeout:  60 * time.Second,
		},
		options:            DefaultOptions(),
		screenshotStrategy: StableScreenshot,
	}
}

// NewDefault returns a new UI Detection automation instance with default params.
func NewDefault(t *chrome.TestConn) *Context {
	return New(t, serverKeyType.Value(), serverKey.Value(), serverKeyAddr.Value())
}

func (uda *Context) copy() *Context {
	return &Context{
		tconn:              uda.tconn,
		detector:           uda.detector,
		pollOpts:           uda.pollOpts,
		options:            uda.options,
		screenshotStrategy: uda.screenshotStrategy,
	}
}

// WithTimeout returns a new Context with the specified timeout.
func (uda *Context) WithTimeout(timeout time.Duration) *Context {
	c := uda.copy()
	c.pollOpts.Timeout = timeout
	return c
}

// WithInterval returns a new Context with the specified polling interval.
func (uda *Context) WithInterval(interval time.Duration) *Context {
	c := uda.copy()
	c.pollOpts.Interval = interval
	return c
}

// WithPollOpts returns a new Context with the specified polling options.
func (uda *Context) WithPollOpts(pollOpts testing.PollOptions) *Context {
	c := uda.copy()
	c.pollOpts = pollOpts
	return c
}

// WithScreenshotStrategy returns a new Context with the specified screenshot strategy.
func (uda *Context) WithScreenshotStrategy(s ScreenshotStrategy) *Context {
	c := uda.copy()
	c.screenshotStrategy = s
	return c
}

// WithOptions returns a new Context with the specified detection options.
func (uda *Context) WithOptions(optionList ...Option) *Context {
	c := uda.copy()
	for _, opt := range optionList {
		opt(c.options)
	}
	return c
}

func (uda *Context) click(s *Finder, button mouse.Button) uiauto.Action {
	// TODO(b/205235148): Consolidate uiauto for UI tree based finder and image based finder.
	return action.Retry(uda.options.Retries, func(ctx context.Context) error {
		return testing.Poll(ctx, func(ctx context.Context) error {
			loc, err := uda.Location(ctx, s)
			if err != nil {
				return errors.Wrapf(err, "failed to find the location of %q", s.desc)
			}

			// Move the mouse over a short duration of time. This fixes the issue where
			// the implementation of `mouse.Click` moves the mouse to the desired location immediately,
			// which prevents applications from registering the move properly. This is especially
			// important in ARC++ applications, which exhibited behavior where two
			// simultaneous clicks needed to be sent since the first would not be
			// registered correctly.
			if err := mouse.Move(uda.tconn, loc.CenterPoint(), 250*time.Millisecond)(ctx); err != nil {
				return errors.Wrap(err, "failed to move the mouse into position")
			}

			return mouse.Click(uda.tconn, loc.CenterPoint(), button)(ctx)
		}, &uda.pollOpts)
	}, uda.options.RetryInterval)
}

// LeftClick returns an action that left-clicks a finder.
func (uda *Context) LeftClick(s *Finder) uiauto.Action {
	return uda.click(s, mouse.LeftButton)
}

// RightClick returns an action that right-clicks a finder.
func (uda *Context) RightClick(s *Finder) uiauto.Action {
	return uda.click(s, mouse.RightButton)
}

// Tap performs a single touchscreen tap.
func (uda *Context) Tap(s *Finder) uiauto.Action {
	return action.Retry(uda.options.Retries, func(ctx context.Context) error {
		return testing.Poll(ctx, func(ctx context.Context) error {
			loc, err := uda.Location(ctx, s)
			if err != nil {
				return errors.Wrapf(err, "failed to find the location of %q", s.desc)
			}

			ts, err := touch.New(ctx, uda.tconn)
			if err != nil {
				return errors.Wrap(err, "failed to create touchscreen")
			}

			return ts.TapAt(loc.CenterPoint())(ctx)
		}, &uda.pollOpts)
	}, uda.options.RetryInterval)
}

// Location finds the location of a finder in the screen.
func (uda *Context) Location(ctx context.Context, s *Finder) (*Location, error) {
	screens, err := display.GetInfo(ctx, uda.tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the display info")
	}

	// Find the ratio to convert coordinates in the screenshot to those in the screen.
	scaleFactor, err := screens[0].GetEffectiveDeviceScaleFactor()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the device scale factor")
	}

	location, err := s.locationPx(ctx, uda, scaleFactor)
	if err != nil {
		return nil, err
	}
	rect := coords.NewRect(location.Left, location.Top, location.Width, location.Height)

	return &Location{
		Text: location.Text,
		Rect: coords.ConvertBoundsFromPXToDP(rect, scaleFactor)}, nil
}

// Exists returns an action that returns nil if the specified element exists.
func (uda *Context) Exists(s *Finder) uiauto.Action {
	return func(ctx context.Context) error {
		testing.ContextLogf(ctx, "Looking for an element %q", s.desc)
		if loc, err := uda.Location(ctx, s); err != nil {
			return err
		} else if loc == nil {
			return errors.Errorf("failed to find element: %q", s.desc)
		} else {
			return nil
		}
	}
}

// WaitUntilExists returns an action that waits until the specified element exists.
func (uda *Context) WaitUntilExists(s *Finder) uiauto.Action {
	return func(ctx context.Context) error {
		return testing.Poll(ctx, uda.Exists(s), &uda.pollOpts)
	}
}

// Gone returns an action that returns nil if the specified element doesn't exist.
func (uda *Context) Gone(s *Finder) uiauto.Action {
	return func(ctx context.Context) error {
		// An not-found error is expected.
		if _, err := uda.Location(ctx, s); err == nil {
			return errors.Errorf("element %q still exists", s.desc)
		} else if !strings.Contains(err.Error(), ErrNotFound) {
			return errors.Errorf("expected error: %q, actual error: %q", ErrNotFound, err)
		}
		return nil
	}
}

// WaitUntilGone returns an action that waits until the specified element doesnt exist.
func (uda *Context) WaitUntilGone(s *Finder) uiauto.Action {
	return func(ctx context.Context) error {
		return testing.Poll(ctx, uda.Gone(s), &uda.pollOpts)
	}
}
