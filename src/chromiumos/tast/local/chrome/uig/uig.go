// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package uig is an action graph library for UI automation on Chrome OS.
// Using uig you create a graph of UI actions to be executed,
// then call Do to execute the graph.
//
// Because the graph construction functions do not return multiple values(eg errors)
// you can freely compose them and handle errors/cleanup/return values only when the
// graph is actually executed. This results in much denser and easier to read UI automation code.
//
// Example:
// 	statusArea := uig.FindWithTimeout(ui.FindParams{ClassName: "ash/StatusAreaWidgetDelegate"}, 10*time.Second)
// 	_, cleanup, err := uig.Steps(
//     statusArea.LeftClick(),
//     uig.FindWithTimeout(ui.FindParams{Name: name, ClassName: "FeaturePodIconButton"}, 10*time.Second).LeftClick()
//     statusArea.LeftClick()).Do(ctx, tconn)
// 	defer cleanup()
// 	if err != nil {
// 		return errors.Wrap(err, "failed to toggle tray setting")
// 	}
package uig

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
)

// Action is a node in a UI automation graph.  Call Do to execute the entire graph starting with the given action.
type Action struct {
	// Execute the graph starting from this node.
	// It returns the underlying ui automation node from the last performed node of the graph, which must be released by the caller.
	// If there is an error, the *ui.Node return will always be nil.
	Do func(ctx context.Context, tconn *chrome.TestConn) (*ui.Node, error)

	// String returns a string representation of the graph starting at this node, typically used in error messages.
	String func() string
}

// WithName gives an action graph an alternative name.
//
// This works best if name resembles a function call, eg:
// func FindColorButton(color string) *Action {
//     return uig.Find(...).
// 		    	  Find(...).
//                WithName("FindColorButton(%q)", color)
// }
//
// Any errors that occur inside this action graph will be wrapped with the
// name, eg:
// file.go:27: got an error: FindColorButton("blue"): uig.Find(...).Find(...): couldn't find element.
//
// Any errors that occur in child actions of this action will have the parent
// actions replaced with the name, eg:
// file.go:27: got an error: FindColorButton("blue").LeftClick(): couldn't click.
func (n *Action) WithName(format string, params ...interface{}) *Action {
	name := fmt.Sprintf(format, params...)
	out := &Action{}
	out.Do = func(ctx context.Context, tconn *chrome.TestConn) (*ui.Node, error) {
		node, err := n.Do(ctx, tconn)
		if err != nil {
			return nil, errors.Wrap(err, name)
		}
		return node, nil
	}
	out.String = func() string {
		return fmt.Sprintf(name)
	}
	return out
}

// Steps combines actions into a sequence of steps that are executed one after another.
func Steps(actions ...*Action) *Action {
	out := &Action{}
	out.Do = func(ctx context.Context, tconn *chrome.TestConn) (*ui.Node, error) {
		var node *ui.Node
		for i, action := range actions {
			if node != nil {
				// We want to return the last node, so we cleanup the node from the previous iteration here
				// instead of the end of the loop.
				node.Release(ctx)
			}
			var err error
			node, err = action.Do(ctx, tconn)
			if err != nil {
				return nil, errors.Wrapf(err, "Step %d", i+1)
			}
		}
		return node, nil
	}
	out.String = func() string {
		var steps []string
		for _, action := range actions {
			steps = append(steps, action.String())
		}
		return fmt.Sprintf("uig.Steps(%s)", strings.Join(steps, ", "))
	}
	return out
}

// FromNode creates an Action from a *ui.Node.
//
// This action simply returns the node when executed and does nothing else.
// The caller is responsible for ensuring that the node remains valid for the
// entire execution of the graph.
//
// The caller is also responsible for cleanup associated with this node, the node
// will not be released as part of the graph's cleanup function.
//
// One use for this is to optimise multiple lookups of the same node, eg:
//
//		statusAreaNode, statusCleanup, err := uig.FindWithTimeout(ui.FindParams{ClassName: "ash/StatusAreaWidgetDelegate"}, 10*time.Second).Do(ctx, tconn)
// 		Defer statusCleanup()
// 		if err != nil {
// 			return errors.Wrap(err, “failed to get statusArea node”)
// 		}
// 		_, cleanup, err := uig.Steps(
//     		uig.FromNode(statusAreaNode).LeftClick(),
//     		uig.FindWithTimeout(ui.FindParams{Name: name, ClassName: "FeaturePodIconButton"}, 10*time.Second).LeftClick()
//     		uig.FromNode(statusAreaNode).LeftClick()).Do(ctx, tconn)
// 		defer cleanup()
// 		if err != nil {
// 			return errors.Wrap(err, "failed to toggle tray setting")
// 		}
func FromNode(node *ui.Node) *Action {
	out := &Action{}
	out.Do = func(ctx context.Context, tconn *chrome.TestConn) (*ui.Node, error) {
		return node, nil
	}
	out.String = func() string {
		return fmt.Sprintf("uig.FromNode(%s:%s:%s)", node.Role, node.ClassName, node.Name)
	}
	return out
}

// LeftClick sends a left mouse click to the screen location of the given node.
//
// Note that if the node is not on the screen it cannot be clicked.  thus you
// may need to call Focus() first.
//
// Also note that if something else is on top of the node (eg a notification)
// that will be clicked instead.
func (n *Action) LeftClick() *Action {
	out := &Action{}
	out.Do = func(ctx context.Context, tconn *chrome.TestConn) (*ui.Node, error) {
		node, err := n.Do(ctx, tconn)
		if err != nil {
			return nil, err
		}
		if err := node.LeftClick(ctx); err != nil {
			node.Release(ctx)
			return nil, errors.Wrap(err, out.String())
		}
		return node, nil
	}
	out.String = func() string { return fmt.Sprintf("%s.LeftClick()", n.String()) }
	return out
}

// RightClick sends a right mouse click to the screen location of the given node.
//
// Note that if the node is not on the screen it cannot be clicked.  thus you
// may need to call Focus() first.
//
// Also note that if something else is on top of the node (eg a notification)
// that will be clicked instead.
func (n *Action) RightClick() *Action {
	out := &Action{}
	out.Do = func(ctx context.Context, tconn *chrome.TestConn) (*ui.Node, error) {
		node, err := n.Do(ctx, tconn)
		if err != nil {
			return nil, err
		}
		if err := node.RightClick(ctx); err != nil {
			node.Release(ctx)
			return nil, errors.Wrap(err, out.String())
		}
		return node, nil
	}
	out.String = func() string { return fmt.Sprintf("%s.RightClick()", n.String()) }
	return out
}

// FocusAndWait puts the keyboard focus on to a node.
//
// An important side effect of this is that it scrolls the node into view,
// which may be necessary before you can call LeftClick on it.
func (n *Action) FocusAndWait(timeout time.Duration) *Action {
	out := &Action{}
	out.Do = func(ctx context.Context, tconn *chrome.TestConn) (*ui.Node, error) {
		node, err := n.Do(ctx, tconn)
		if err != nil {
			return nil, err
		}
		if err := node.FocusAndWait(ctx, timeout); err != nil {
			node.Release(ctx)
			return nil, errors.Wrap(err, out.String())
		}
		return node, nil
	}
	out.String = func() string { return fmt.Sprintf("%s.Focus()", n.String()) }
	return out
}

// Find finds a descendant of the node it is called on.
func (n *Action) Find(params ui.FindParams) *Action {
	out := &Action{}
	out.Do = func(ctx context.Context, tconn *chrome.TestConn) (*ui.Node, error) {
		parent, err := n.Do(ctx, tconn)
		if err != nil {
			return nil, err
		}
		node, err := parent.Descendant(ctx, params)
		parent.Release(ctx)
		if err != nil {
			return nil, errors.Wrap(err, out.String())
		}
		return node, nil
	}
	out.String = func() string { return fmt.Sprintf("%s.Find(%v)", n.String(), params) }
	return out
}

// Find is a shortcut for uig.Root().Find(...).
func Find(params ui.FindParams) *Action {
	return Root().Find(params).WithName("uig.Find(%v)", params)
}

// FindWithTimeout finds a descendant of the node it is called on. It returns an error if the timeout expires.
func (n *Action) FindWithTimeout(params ui.FindParams, timeout time.Duration) *Action {
	out := &Action{}
	out.Do = func(ctx context.Context, tconn *chrome.TestConn) (*ui.Node, error) {
		parent, err := n.Do(ctx, tconn)
		if err != nil {
			return nil, err
		}
		node, err := parent.DescendantWithTimeout(ctx, params, timeout)
		parent.Release(ctx)
		if err != nil {
			return nil, errors.Wrap(err, out.String())
		}
		return node, nil
	}
	out.String = func() string { return fmt.Sprintf("%s.Find(%v)", n.String(), params) }
	return out
}

// FindWithTimeout is a shortcut for uig.Root().FindWithTimeout(...).
func FindWithTimeout(params ui.FindParams, timeout time.Duration) *Action {
	return Root().FindWithTimeout(params, timeout).WithName("uig.FindWithTimeout(%v, %v)", params, timeout)
}

// WaitUntilDescendantExists waits until a given node is found as a descendant
// of the node this is called on.
//
// If the timeout expires while the node does not exist it will return error.
func (n *Action) WaitUntilDescendantExists(params ui.FindParams, timeout time.Duration) *Action {
	out := &Action{}
	out.Do = func(ctx context.Context, tconn *chrome.TestConn) (*ui.Node, error) {
		parent, err := n.Do(ctx, tconn)
		if err != nil {
			return nil, err
		}
		err = parent.WaitUntilDescendantExists(ctx, params, timeout)
		if err != nil {
			parent.Release(ctx)
			return nil, errors.Wrap(err, out.String())
		}
		return parent, nil
	}
	out.String = func() string { return fmt.Sprintf("%s.WaitUntilDescendantExists(%v, %v)", n.String(), params, timeout) }
	return out
}

// WaitUntilDescendantExists is a shortcut for uig.Root().WaitUntilDescendantExists(...)
func WaitUntilDescendantExists(params ui.FindParams, timeout time.Duration) *Action {
	return Root().WaitUntilDescendantExists(params, timeout).WithName("uig.WaitUntilDescendantExists(%v, %v)", params, timeout)
}

// WaitUntilDescendantGone waits until a given node can no longer be found as a descendant
// of the node this is called on.
//
// If the timeout expires while the node still exists it will return error.
func (n *Action) WaitUntilDescendantGone(params ui.FindParams, timeout time.Duration) *Action {
	out := &Action{}
	out.Do = func(ctx context.Context, tconn *chrome.TestConn) (*ui.Node, error) {
		parent, err := n.Do(ctx, tconn)
		if err != nil {
			return nil, err
		}
		err = parent.WaitUntilDescendantGone(ctx, params, timeout)
		if err != nil {
			parent.Release(ctx)
			return nil, errors.Wrap(err, out.String())
		}
		return parent, nil
	}
	out.String = func() string { return fmt.Sprintf("%s.WaitUntilDescendantGone(%v, %v)", n.String(), params, timeout) }
	return out
}

// WaitUntilDescendantGone is a shortcut for uig.Root().WaitUntilDescendantGone(...)
func WaitUntilDescendantGone(params ui.FindParams, timeout time.Duration) *Action {
	return Root().WaitUntilDescendantGone(params, timeout).WithName("uig.WaitUntilDescendantGone(%v, %v)", params, timeout)
}

// Root gets the ChromeOS Desktop.
func Root() *Action {
	out := &Action{}
	out.Do = func(ctx context.Context, tconn *chrome.TestConn) (*ui.Node, error) {
		root, err := ui.Root(ctx, tconn)
		if err != nil {
			return nil, errors.Wrap(err, out.String())
		}
		return root, nil
	}
	out.String = func() string { return "uig.Root()" }
	return out
}

// Do is a convenience wrapper for Action.Do.
// It automatically releases the returned *ui.Node as needed.
func Do(ctx context.Context, tconn *chrome.TestConn, graph *Action) error {
	node, err := graph.Do(ctx, tconn)
	if err != nil {
		return err
	}
	node.Release(ctx)
	return nil
}
