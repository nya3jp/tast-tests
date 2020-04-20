// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package uic is a combinator/graph library for UI automation on Chrome OS.
// Using uic you create a graph of actions to be UI actions to be executed,
// then call Do to execute the graph.
//
// Because the graph construction functions do not return multiple values(eg errors)
// you can freely compose them and handle errors/cleanup/return values only when the
// graph is actually executed. This results in much denser and easier to read UI automation code.
package uic

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/testing"
)

// CleanupFunc cleans up anything left over from execution of a UI automation graph.
type CleanupFunc func()

// Node is a node in a UI automation graph.  Call Do to execute the entire graph.
type Node struct {
	// Execute the graph starting from this node.
	// It returns the underlying ui automation node from the last performed node of the graph,
	// a cleanup function that must be called (usually with defer) and any errors that occurred.
	Do func(ctx context.Context, tconn *chrome.TestConn) (*ui.Node, CleanupFunc, error)

	//String returns a string representation of the graph starting at this node, typically used in error mesages.
	String func() string
}

func noCleanup() {}

// Root gets the node representing the ChromeOS Desktop.
func Root() *Node {
	out := &Node{}
	out.Do = func(ctx context.Context, tconn *chrome.TestConn) (*ui.Node, CleanupFunc, error) {
		root, err := ui.Root(ctx, tconn)
		if err != nil {
			return nil, noCleanup, errors.Wrap(err, out.String())
		}
		cleanup := func() {
			root.Release(ctx)
		}
		return root, cleanup, err
	}
	out.String = func() string { return "uic.Root()" }
	return out
}

// Wait waits for the given timeout before executing the next UI automation action.
func (n *Node) Wait(d time.Duration) *Node {
	out := &Node{}
	out.Do = func(ctx context.Context, tconn *chrome.TestConn) (*ui.Node, CleanupFunc, error) {
		node, cleanup, err := n.Do(ctx, tconn)
		if err != nil {
			cleanup()
			return nil, noCleanup, errors.Wrap(err, out.String())
		}
		testing.Sleep(d)
		return node, cleanup, nil
	}
	out.String = func() string { return fmt.Sprintf("%s.Wait(%v)", n.String(), d) }
	return out
}

// Find finds a node that is a descendant of the node it is called on.
func (n *Node) Find(params ui.FindParams) *Node {
	out := &Node{}
	out.Do = func(ctx context.Context, tconn *chrome.TestConn) (*ui.Node, CleanupFunc, error) {
		parent, cleanup, err := n.Do(ctx, tconn)
		if err != nil {
			cleanup()
			return nil, noCleanup, err
		}
		node, err := parent.DescendantWithTimeout(ctx, params, 5*time.Second)
		if err != nil {
			cleanup()
			return nil, noCleanup, errors.Wrap(err, out.String())
		}
		nodeCleanup := func() {
			node.Release(ctx)
			cleanup()
		}
		return node, nodeCleanup, nil
	}
	out.String = func() string { return fmt.Sprintf("%s.Find(%v)", n.String(), params) }
	return out
}

// Find is a shortcut for uic.Root().Find(...).
func Find(params ui.FindParams) *Node {
	return Root().Find(params)
}

// WaitUntilGone waits for a given node can no longer be found as a descendant of the node this is called on.
// If the timeout expires while the node still exists it will return error.
func (n *Node) WaitUntilGone(params ui.FindParams, timeout time.Duration) *Node {
	out := &Node{}
	out.Do = func(ctx context.Context, tconn *chrome.TestConn) (*ui.Node, CleanupFunc, error) {
		parent, cleanup, err := n.Do(ctx, tconn)
		if err != nil {
			cleanup()
			return nil, noCleanup, err
		}
		err = parent.WaitUntilDescendantGone(ctx, params, timeout)
		if err != nil {
			cleanup()
			return nil, noCleanup, errors.Wrap(err, out.String())
		}
		return parent, cleanup, nil
	}
	out.String = func() string { return fmt.Sprintf("%s.WaitUntilGone(%v, %v)", n.String(), params, timeout) }
	return out
}

// WaitUntilGone is a shortcut for uic.Root().WaitUntilGone(...)
func WaitUntilGone(params ui.FindParams, timeout time.Duration) *Node {
	return Root().WaitUntilGone(params, timeout)
}

// LeftClick sends a left mouse click to the screen location of the given node.
// Note that if the node is not on the screen it cannot be clicked.  thus you may need to call Focus() first.
// Also note that if something else is on top of the node (eg a notification) that will be clicked instead.
func (n *Node) LeftClick() *Node {
	out := &Node{}
	out.Do = func(ctx context.Context, tconn *chrome.TestConn) (*ui.Node, CleanupFunc, error) {
		node, cleanup, err := n.Do(ctx, tconn)
		if err != nil {
			cleanup()
			return nil, noCleanup, err
		}
		if err := node.LeftClick(ctx); err != nil {
			return node, cleanup, errors.Wrap(err, out.String())
		}
		return node, cleanup, nil
	}
	out.String = func() string { return fmt.Sprintf("%s.LeftClick()", n.String()) }
	return out
}

// Focus puts the keyboard focus on to a node.  An important side effect of this is that it scrolls the
// node into view, which may be necessary before you can call LeftClick on it.
func (n *Node) Focus() *Node {
	out := &Node{}
	out.Do = func(ctx context.Context, tconn *chrome.TestConn) (*ui.Node, CleanupFunc, error) {
		node, cleanup, err := n.Do(ctx, tconn)
		if err != nil {
			cleanup()
			return nil, noCleanup, err
		}
		if err := node.FocusAndWait(ctx, 5*time.Second); err != nil {
			return node, cleanup, errors.Wrap(err, out.String())
		}
		return node, cleanup, nil
	}
	out.String = func() string { return fmt.Sprintf("%s.Focus()", n.String()) }
	return out
}

// Launch launches an app given by appId.  The name parameter is only used in error messages.
// Note that when the cleanup function for this node is called it will close the app.
func Launch(name, appID string) *Node {
	out := &Node{}
	out.Do = func(ctx context.Context, tconn *chrome.TestConn) (*ui.Node, CleanupFunc, error) {
		if err := apps.Launch(ctx, tconn, appID); err != nil {
			return nil, noCleanup, errors.Wrapf(err, "%s: Failed to launch app", out.String())
		}
		if err := ash.WaitForApp(ctx, tconn, appID); err != nil {
			return nil, noCleanup, errors.Wrapf(err, "%s: app did not appear in the shelf", out.String())
		}

		// It would be better if we could return the main window of the app that just opened...
		root, err := ui.Root(ctx, tconn)
		if err != nil {
			apps.Close(ctx, tconn, appID)
			return nil, noCleanup, errors.Wrapf(err, "%s: unable to find root:", out.String())
		}
		cleanup := func() {
			root.Release(ctx)
			apps.Close(ctx, tconn, appID)
		}
		return root, cleanup, nil
	}
	out.String = func() string { return fmt.Sprintf("uic.Launch(%s, %s)", name, appID) }
	return out
}

// Steps combines nodes into a sequence of steps that are executed one after another.
func Steps(nodes ...*Node) *Node {
	out := &Node{}
	out.Do = func(ctx context.Context, tconn *chrome.TestConn) (*ui.Node, CleanupFunc, error) {
		var cleanups []CleanupFunc
		cleanupAll := func() {
			for _, c := range cleanups {
				c()
			}
		}
		var node *ui.Node
		for i, n := range nodes {
			var err error
			var cleanup CleanupFunc
			node, cleanup, err = n.Do(ctx, tconn)
			cleanups = append([]CleanupFunc{cleanup}, cleanups...)
			if err != nil {
				cleanupAll()
				return nil, noCleanup, errors.Wrapf(err, "Step %d", i+1)
			}
		}
		return node, cleanupAll, nil
	}
	out.String = func() string {
		var steps []string
		for _, n := range nodes {
			steps = append(steps, n.String())
		}
		return fmt.Sprintf("uic.Steps(%s)", strings.Join(steps, ", "))
	}
	return out
}
