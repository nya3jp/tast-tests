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
//  statusArea := uig.FindWithTimeout(ui.FindParams{ClassName: "ash/StatusAreaWidgetDelegate"}, 10*time.Second)
//  steps := uig.Steps(
//     statusArea.LeftClick(),
//     uig.FindWithTimeout(ui.FindParams{Name: name, ClassName: "FeaturePodIconButton"}, 10*time.Second).LeftClick()
//     statusArea.LeftClick())
//  if err := uig.Do(ctx, tconn, steps); err != nil {
//     return errors.Wrap(err, "failed to toggle tray setting")
//  }
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
	// The parent node should be passed through to the parent action.  It will eventually reach uig.ParentOrRoot() or a similar function.
	// It returns the underlying ui automation node from the last performed node of the graph, which must be released by the caller.
	// If there is an error, the *ui.Node return will always be nil.
	// If there is no error, the *ui.Node will never be nil.  If you are implementing a custom action and have nothing sensible to return,
	// return the node from uig.ParentOrRoot().
	do func(ctx context.Context, tconn *chrome.TestConn, parent *ui.Node) (*ui.Node, error)

	// name is a string representation of the graph starting at this node, typically used in error messages.
	name string
}

// String returns a string representation of the graph starting at this node.
func (a *Action) String() string {
	return a.name
}

// WithName gives an action graph an alternative name.
//
// This works best if name resembles a function call, eg:
// func FindColorButton(color string) *Action {
//     return uig.Find(...).
//         Find(...).
//         WithName("FindColorButton(%q)", color)
// }
//
// Any errors that occur inside this action graph will be wrapped with the
// name, eg:
// file.go:27: got an error: FindColorButton("blue"): uig.Find(...).Find(...): couldn't find element.
//
// Any errors that occur in child actions of this action will have the parent
// actions replaced with the name, eg:
// file.go:27: got an error: FindColorButton("blue").LeftClick(): couldn't click.
func (a *Action) WithName(format string, params ...interface{}) *Action {
	name := fmt.Sprintf(format, params...)
	return &Action{
		name: name,
		do: func(ctx context.Context, tconn *chrome.TestConn, parent *ui.Node) (*ui.Node, error) {
			node, err := a.do(ctx, tconn, parent)
			if err != nil {
				return nil, errors.Wrap(err, name)
			}
			return node, nil
		},
	}

}

// Steps combines actions into a sequence of steps that are executed one after another.
func (a *Action) Steps(actions ...*Action) *Action {
	var steps []string
	for _, action := range actions {
		steps = append(steps, action.String())
	}
	name := fmt.Sprintf("%v.Steps(%s)", a, strings.Join(steps, ", "))

	return &Action{
		name: name,
		do: func(ctx context.Context, tconn *chrome.TestConn, parent *ui.Node) (*ui.Node, error) {
			node, err := a.do(ctx, tconn, parent)
			if err != nil {
				return nil, errors.Wrap(err, name)
			}
			for i, action := range actions {
				child, err := action.do(ctx, tconn, node)
				if err != nil {
					if len(actions) > 1 {
						return nil, errors.Wrapf(err, "Step %d", i+1)
					}
					return nil, err
				}
				child.Release(ctx)
			}
			return node, nil
		},
	}
}

// Steps combines actions into a sequence of steps that are executed one after another.
func Steps(actions ...*Action) *Action {
	return Root().Steps(actions...)
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
//      statusAreaNode, err := uig.FindWithTimeout(ui.FindParams{ClassName: "ash/StatusAreaWidgetDelegate"}, 10*time.Second).Do(ctx, tconn)
//      if err != nil {
//          return errors.Wrap(err, “failed to get statusArea node”)
//      }
//      defer statusAreaNode.Release()
//      steps := uig.Steps(
//      uig.FromNode(statusAreaNode).LeftClick(),
//          uig.FindWithTimeout(ui.FindParams{Name: name, ClassName: "FeaturePodIconButton"}, 10*time.Second).LeftClick()
//          uig.FromNode(statusAreaNode).LeftClick())
//      if uig.Do(ctx, tconn, steps); err != nil {
//          return errors.Wrap(err, "failed to toggle tray setting")
//      }
func FromNode(node *ui.Node) *Action {
	return &Action{
		name: fmt.Sprintf("uig.FromNode(%s:%s:%s)", node.Role, node.ClassName, node.Name),
		do: func(ctx context.Context, tconn *chrome.TestConn, parent *ui.Node) (*ui.Node, error) {
			return node, nil
		},
	}
}

// LeftClick sends a left mouse click to the screen location of the given node.
//
// Note that if the node is not on the screen it cannot be clicked.  thus you
// may need to call Focus() first.
//
// Also note that if something else is on top of the node (eg a notification)
// that will be clicked instead.
func (a *Action) LeftClick() *Action {
	name := fmt.Sprintf("%s.LeftClick()", a.String())
	return &Action{
		name: name,
		do: func(ctx context.Context, tconn *chrome.TestConn, parent *ui.Node) (*ui.Node, error) {
			node, err := a.do(ctx, tconn, parent)
			if err != nil {
				return nil, err
			}
			if err := node.LeftClick(ctx); err != nil {
				node.Release(ctx)
				return nil, errors.Wrap(err, name)
			}
			return node, nil
		},
	}
}

// RightClick sends a right mouse click to the screen location of the given node.
//
// Note that if the node is not on the screen it cannot be clicked.  thus you
// may need to call Focus() first.
//
// Also note that if something else is on top of the node (eg a notification)
// that will be clicked instead.
func (a *Action) RightClick() *Action {
	name := fmt.Sprintf("%s.RightClick()", a.String())
	return &Action{
		name: name,
		do: func(ctx context.Context, tconn *chrome.TestConn, parent *ui.Node) (*ui.Node, error) {
			node, err := a.do(ctx, tconn, parent)
			if err != nil {
				return nil, err
			}
			if err := node.RightClick(ctx); err != nil {
				node.Release(ctx)
				return nil, errors.Wrap(err, name)
			}
			return node, nil
		},
	}
}

// FocusAndWait puts the keyboard focus on to a node.
//
// An important side effect of this is that it scrolls the node into view,
// which may be necessary before you can call LeftClick on it.
func (a *Action) FocusAndWait(timeout time.Duration) *Action {
	name := fmt.Sprintf("%s.Focus()", a.String())
	return &Action{
		name: name,
		do: func(ctx context.Context, tconn *chrome.TestConn, parent *ui.Node) (*ui.Node, error) {
			node, err := a.do(ctx, tconn, parent)
			if err != nil {
				return nil, err
			}
			if err := node.FocusAndWait(ctx, timeout); err != nil {
				node.Release(ctx)
				return nil, errors.Wrap(err, name)
			}
			return node, nil
		},
	}
}

// Find finds a descendant of the node it is called on.
func (a *Action) Find(params ui.FindParams) *Action {
	name := fmt.Sprintf("%s.Find(%v)", a.String(), params)
	return &Action{
		name: name,
		do: func(ctx context.Context, tconn *chrome.TestConn, parent *ui.Node) (*ui.Node, error) {
			node, err := a.do(ctx, tconn, parent)
			if err != nil {
				return nil, err
			}
			child, err := node.Descendant(ctx, params)
			node.Release(ctx)
			if err != nil {
				return nil, errors.Wrap(err, name)
			}
			return child, nil
		},
	}
}

// Find is a shortcut for uig.Root().Find(...).
func Find(params ui.FindParams) *Action {
	return ParentOrRoot().Find(params).WithName("uig.Find(%v)", params)
}

// FindWithTimeout finds a descendant of the node it is called on. It returns an error if the timeout expires.
func (a *Action) FindWithTimeout(params ui.FindParams, timeout time.Duration) *Action {
	name := fmt.Sprintf("%s.Find(%v)", a.String(), params)
	return &Action{
		name: name,
		do: func(ctx context.Context, tconn *chrome.TestConn, parent *ui.Node) (*ui.Node, error) {
			node, err := a.do(ctx, tconn, parent)
			if err != nil {
				return nil, err
			}
			child, err := node.DescendantWithTimeout(ctx, params, timeout)
			node.Release(ctx)
			if err != nil {
				return nil, errors.Wrap(err, name)
			}
			return child, nil
		},
	}
}

// FindWithTimeout is a shortcut for uig.Root().FindWithTimeout(...).
func FindWithTimeout(params ui.FindParams, timeout time.Duration) *Action {
	return ParentOrRoot().FindWithTimeout(params, timeout).WithName("uig.FindWithTimeout(%v, %v)", params, timeout)
}

// WaitUntilDescendantExists waits until a given node is found as a descendant
// of the node this is called on.
//
// If the timeout expires while the node does not exist it will return error.
func (a *Action) WaitUntilDescendantExists(params ui.FindParams, timeout time.Duration) *Action {
	name := fmt.Sprintf("%s.WaitUntilDescendantExists(%v, %v)", a.String(), params, timeout)
	return &Action{
		name: name,
		do: func(ctx context.Context, tconn *chrome.TestConn, parent *ui.Node) (*ui.Node, error) {
			node, err := a.do(ctx, tconn, parent)
			if err != nil {
				return nil, err
			}
			err = node.WaitUntilDescendantExists(ctx, params, timeout)
			if err != nil {
				node.Release(ctx)
				return nil, errors.Wrap(err, name)
			}
			return node, nil
		},
	}
}

// WaitUntilDescendantExists is a shortcut for uig.Root().WaitUntilDescendantExists(...)
func WaitUntilDescendantExists(params ui.FindParams, timeout time.Duration) *Action {
	return ParentOrRoot().WaitUntilDescendantExists(params, timeout).WithName("uig.WaitUntilDescendantExists(%v, %v)", params, timeout)
}

// WaitUntilDescendantGone waits until a given node can no longer be found as a descendant
// of the node this is called on.
//
// If the timeout expires while the node still exists it will return error.
func (a *Action) WaitUntilDescendantGone(params ui.FindParams, timeout time.Duration) *Action {
	name := fmt.Sprintf("%s.WaitUntilDescendantGone(%v, %v)", a.String(), params, timeout)
	return &Action{
		name: name,
		do: func(ctx context.Context, tconn *chrome.TestConn, parent *ui.Node) (*ui.Node, error) {
			node, err := a.do(ctx, tconn, parent)
			if err != nil {
				return nil, err
			}
			err = node.WaitUntilDescendantGone(ctx, params, timeout)
			if err != nil {
				node.Release(ctx)
				return nil, errors.Wrap(err, name)
			}
			return node, nil
		},
	}
}

// WaitUntilDescendantGone is a shortcut for uig.Root().WaitUntilDescendantGone(...)
func WaitUntilDescendantGone(params ui.FindParams, timeout time.Duration) *Action {
	return ParentOrRoot().WaitUntilDescendantGone(params, timeout).WithName("uig.WaitUntilDescendantGone(%v, %v)", params, timeout)
}

// Root gets the ChromeOS Desktop.
func Root() *Action {
	name := "uig.Root()"
	return &Action{
		name: name,
		do: func(ctx context.Context, tconn *chrome.TestConn, parent *ui.Node) (*ui.Node, error) {
			if parent != nil {
				parent.Release(ctx)
			}
			root, err := ui.Root(ctx, tconn)
			if err != nil {
				return nil, errors.Wrap(err, name)
			}
			return root, nil
		},
	}
}

// ParentOrRoot gets the parent for the current context (eg if Steps was called on a non-root node)
// or the root ChromeOS Desktop node if no contextual parent exists.
func ParentOrRoot() *Action {
	name := "uig.ParentOrRoot()"
	return &Action{
		name: name,
		do: func(ctx context.Context, tconn *chrome.TestConn, parent *ui.Node) (*ui.Node, error) {
			if parent != nil {
				return parent, nil
			}
			root, err := ui.Root(ctx, tconn)
			if err != nil {
				return nil, errors.Wrap(err, name)
			}
			return root, nil
		},
	}
}

// GetNode executes an action graph.  It returns the ui node from the last action.
// The caller is responsible for calling Release() on the returned *ui.Node.
func GetNode(ctx context.Context, tconn *chrome.TestConn, graph *Action) (*ui.Node, error) {
	node, err := graph.do(ctx, tconn, nil)
	if err != nil {
		return nil, err
	}
	return node, nil
}

// Do executes one or more action graphs in sequence.  It automatically releases any resources as required.
func Do(ctx context.Context, tconn *chrome.TestConn, graphs ...*Action) error {
	node, err := Steps(graphs...).do(ctx, tconn, nil)
	if err != nil {
		return err
	}
	node.Release(ctx)
	return nil
}
