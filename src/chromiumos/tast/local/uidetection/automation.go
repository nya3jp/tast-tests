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
	"chromiumos/tast/local/input"
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

// LeftClickVirtualMouse Performs a left click using a virtual mouse.
// The difference between this method and `LeftClick` is that this method
// will generate a virtual hardware element that simulates the events a real
// mouse would send, and behaves like a user when performing the click, i.e.
// taking time to move the mouse into position, and then a temporary pause
// before the click is performed. This level of emulation is required for certain
// applications (especially in ARC++), and better emulates a user click,
// without relying on internal methods in the private chrome extension which
// may bypass events that an application is looking for (i.e. a hover before
// a click).
func (uda *Context) LeftClickVirtualMouse(s *Finder) uiauto.Action {
	const (
		moveTime             = 250 * time.Millisecond
		pauseBeforeClickTime = time.Second
	)

	return func(ctx context.Context) error {
		// Find the location of the element
		var loc *Location
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			// Get the location of the element.
			var err error
			loc, err = uda.Location(ctx, s)
			if err != nil {
				return errors.Wrapf(err, "failed to find the location of %q", s.desc)
			}

			return nil
		}, &uda.pollOpts); err != nil {
			return errors.Wrap(err, "failed to poll for the element location")
		}

		// Create a virtual mouse.
		mew, err := input.Mouse(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to create the mouse")
		}
		defer mew.Close()

		// Move the mouse into position over a period of time to simulate real movement.
		if err := mouse.Move(uda.tconn, loc.CenterPoint(), moveTime)(ctx); err != nil {
			return errors.Wrap(err, "failed to move the mouse")
		}

		// Pause temporarily before performing a click so that the application can register
		// the position of the mouse.
		if err := testing.Sleep(ctx, pauseBeforeClickTime); err != nil {
			return errors.Wrap(err, "failed to sleep before the mouse click")
		}

		// Send the click event.
		if err := mew.Click(); err != nil {
			return errors.Wrap(err, "failed to perform the click")
		}

		return nil
	}
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
