// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cuj has utilities for CUJ-style UI performance tests.
package cuj

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// InputAction define UI click action on a ui.Node.
type InputAction interface {
	LeftClick(ctx context.Context, n *ui.Node) error
	RightClick(ctx context.Context, n *ui.Node) error
	DoubleClick(ctx context.Context, n *ui.Node) error
	StableLeftClick(ctx context.Context, n *ui.Node, pollOpts *testing.PollOptions) error
	StableRightClick(ctx context.Context, n *ui.Node, pollOpts *testing.PollOptions) error
	StableDoubleClick(ctx context.Context, n *ui.Node, pollOpts *testing.PollOptions) error
	StableFindAndClick(ctx context.Context, tconn *chrome.TestConn, params ui.FindParams, opts *testing.PollOptions) error
	StableFindAndRightClick(ctx context.Context, tconn *chrome.TestConn, params ui.FindParams, opts *testing.PollOptions) error
}

// MouseActionHandler implements the ui.Action by using mouse event
type MouseActionHandler struct {
}

// TouchActionHandler implements the ui.Action by using touch event
type TouchActionHandler struct {
	stw *input.SingleTouchEventWriter
	tcc *input.TouchCoordConverter
}

// NewMouseActionHandler return the ActionHandler by using mouse event
func NewMouseActionHandler() *MouseActionHandler {
	return &MouseActionHandler{}
}

// NewTouchActionHandler return the ActionHandler by using touch event
func NewTouchActionHandler(stw *input.SingleTouchEventWriter, tcc *input.TouchCoordConverter) *TouchActionHandler {
	return &TouchActionHandler{
		stw: stw,
		tcc: tcc,
	}
}

// LeftClick executes a left click on the node.
// If the JavaScript fails to execute, an error is returned.
func (m *MouseActionHandler) LeftClick(ctx context.Context, n *ui.Node) error {
	return n.LeftClick(ctx)
}

// RightClick executes a right click on the node.
// If the JavaScript fails to execute, an error is returned.
func (m *MouseActionHandler) RightClick(ctx context.Context, n *ui.Node) error {
	return n.RightClick(ctx)
}

// DoubleClick executes 2 mouse left clicks on the node.
// If the JavaScript fails to execute, an error is returned.
func (m *MouseActionHandler) DoubleClick(ctx context.Context, n *ui.Node) error {
	return n.DoubleClick(ctx)
}

// StableLeftClick waits for the location to be stable and then left clicks the node.
// The location must be stable for 1 iteration of polling (default 100ms).
func (m *MouseActionHandler) StableLeftClick(ctx context.Context, n *ui.Node, pollOpts *testing.PollOptions) error {
	return n.StableLeftClick(ctx, pollOpts)
}

// StableRightClick waits for the location to be stable and then right clicks the node.
// The location must be stable for 1 iteration of polling (default 100ms).
func (m *MouseActionHandler) StableRightClick(ctx context.Context, n *ui.Node, pollOpts *testing.PollOptions) error {
	return n.StableRightClick(ctx, pollOpts)
}

// StableDoubleClick waits for the location to be stable and then double clicks the node.
// The location must be stable for 1 iteration of polling (default 100ms).
func (m *MouseActionHandler) StableDoubleClick(ctx context.Context, n *ui.Node, pollOpts *testing.PollOptions) error {
	return n.StableDoubleClick(ctx, pollOpts)
}

// StableFindAndClick waits for the first matching stable node and then left clicks it.
func (m *MouseActionHandler) StableFindAndClick(ctx context.Context, tconn *chrome.TestConn, params ui.FindParams, opts *testing.PollOptions) error {
	node, err := ui.StableFind(ctx, tconn, params, opts)
	if err != nil {
		return errors.Wrapf(err, "failed to find stable node with %v", params)
	}
	defer node.Release(ctx)
	return node.LeftClick(ctx)
}

// StableFindAndRightClick waits for the first matching stable node and then right clicks it.
func (m *MouseActionHandler) StableFindAndRightClick(ctx context.Context, tconn *chrome.TestConn, params ui.FindParams, opts *testing.PollOptions) error {
	node, err := ui.StableFind(ctx, tconn, params, opts)
	if err != nil {
		return errors.Wrapf(err, "failed to find stable node with %v", params)
	}
	defer node.Release(ctx)
	return node.RightClick(ctx)
}

// LeftClick executes a left click on the node.
// If the JavaScript fails to execute, an error is returned.
func (t *TouchActionHandler) LeftClick(ctx context.Context, n *ui.Node) error {
	return touch(ctx, n, t.stw, t.tcc)
}

// RightClick executes a right click on the node.
// Currently the right click action is mapped to long touch on touch screen.
func (t *TouchActionHandler) RightClick(ctx context.Context, n *ui.Node) error {
	return longTouch(ctx, n, t.stw, t.tcc)
}

// DoubleClick executes 2 mouse left clicks on the node.
// Currently there is no match action of mouse double click on touch screen.
func (t *TouchActionHandler) DoubleClick(ctx context.Context, n *ui.Node) error {
	return nil
}

// StableLeftClick waits for the location to be stable and then left clicks the node.
// The location must be stable for 1 iteration of polling (default 100ms).
func (t *TouchActionHandler) StableLeftClick(ctx context.Context, n *ui.Node, pollOpts *testing.PollOptions) error {
	return stableTouch(ctx, n, t.stw, t.tcc, pollOpts)
}

// StableRightClick waits for the location to be stable and then right clicks the node.
// The location must be stable for 1 iteration of polling (default 100ms).
// Currently the right click action is mapped to long touch on touch screen.
func (t *TouchActionHandler) StableRightClick(ctx context.Context, n *ui.Node, pollOpts *testing.PollOptions) error {
	return stableLongTouch(ctx, n, t.stw, t.tcc, pollOpts)
}

// StableDoubleClick waits for the location to be stable and then double clicks the node.
// The location must be stable for 1 iteration of polling (default 100ms).
// Currently there is no match action of mouse double click on touch screen.
func (t *TouchActionHandler) StableDoubleClick(ctx context.Context, n *ui.Node, pollOpts *testing.PollOptions) error {
	return nil
}

// StableFindAndClick waits for the first matching stable node and then left clicks it.
func (t *TouchActionHandler) StableFindAndClick(ctx context.Context, tconn *chrome.TestConn, params ui.FindParams, opts *testing.PollOptions) error {
	node, err := ui.StableFind(ctx, tconn, params, opts)
	if err != nil {
		return errors.Wrapf(err, "failed to find stable node with %v", params)
	}
	defer node.Release(ctx)
	return t.LeftClick(ctx, node)
}

// StableFindAndRightClick waits for the first matching stable node and then right clicks it.
func (t *TouchActionHandler) StableFindAndRightClick(ctx context.Context, tconn *chrome.TestConn, params ui.FindParams, opts *testing.PollOptions) error {
	node, err := ui.StableFind(ctx, tconn, params, opts)
	if err != nil {
		return errors.Wrapf(err, "failed to find stable node with %v", params)
	}
	defer node.Release(ctx)
	return t.RightClick(ctx, node)
}

// stableTouch waits for the location to be stable and then touch the node.
// The location must be stable for 1 iteration of polling (default 100ms).
func stableTouch(ctx context.Context, n *ui.Node, stw *input.SingleTouchEventWriter, tcc *input.TouchCoordConverter, pollOpts *testing.PollOptions) error {
	if err := n.WaitLocationStable(ctx, pollOpts); err != nil {
		return err
	}
	return touch(ctx, n, stw, tcc)
}

// stableLongTouch waits for the location to be stable and then long-touch the node.
// The location must be stable for 1 iteration of polling (default 100ms).
func stableLongTouch(ctx context.Context, n *ui.Node, stw *input.SingleTouchEventWriter, tcc *input.TouchCoordConverter, pollOpts *testing.PollOptions) error {
	if err := n.WaitLocationStable(ctx, pollOpts); err != nil {
		return err
	}
	return longTouch(ctx, n, stw, tcc)
}

// touch executes a touch event on the node.
func touch(ctx context.Context, n *ui.Node, stw *input.SingleTouchEventWriter, tcc *input.TouchCoordConverter) error {
	x, y := tcc.ConvertLocation(n.Location.CenterPoint())
	if err := stw.Move(x, y); err != nil {
		return err
	}
	return stw.End()
}

// longTouch executes a long-touch event on the node.
func longTouch(ctx context.Context, n *ui.Node, stw *input.SingleTouchEventWriter, tcc *input.TouchCoordConverter) error {
	x, y := tcc.ConvertLocation(n.Location.CenterPoint())
	if err := stw.LongPressAt(ctx, x, y); err != nil {
		return err
	}
	return stw.End()
}
