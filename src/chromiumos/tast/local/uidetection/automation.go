// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package uidetection

import (
	"context"

	"chromiumos/tast/common/action"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/coords"
)

// Context provides functionalities for image-based UI automation.
type Context struct {
	tconn    *chrome.TestConn
	detector *uiDetector
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
	}
}

func (uda *Context) click(s *Selector, button mouse.Button, optionList ...Option) uiauto.Action {
	options := DefaulOptions()
	for _, opt := range optionList {
		opt(options)
	}
	return action.Retry(options.Retries, func(ctx context.Context) error {
		loc, err := uda.Location(ctx, s)
		if err != nil {
			return errors.Wrapf(err, "failed to find the location of %q", s.desc)
		}
		return mouse.Click(
			uda.tconn,
			coords.NewPoint((loc.TopLeft.X+loc.BottomRight.X)/2, (loc.TopLeft.Y+loc.BottomRight.Y)/2),
			button,
		)(ctx)
	}, options.RetryInterval)
}

// LeftClick returns an action that left-clicks a selector.
func (uda *Context) LeftClick(s *Selector, optionList ...Option) uiauto.Action {
	return uda.click(s, mouse.LeftButton, optionList...)
}

// RightClick returns an action that right-clicks a selector.
func (uda *Context) RightClick(s *Selector, optionList ...Option) uiauto.Action {
	return uda.click(s, mouse.RightButton, optionList...)
}

// Location finds the location of a selector in the screen.
func (uda *Context) Location(ctx context.Context, s *Selector) (*Location, error) {
	if err := s.resolve(ctx, uda.detector); err != nil {
		return nil, errors.Wrapf(err, "failed to resolve the selector: %q", s.desc)
	}
	return s.location()
}
