// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package uiauto enables automating with the ChromeOS UI through the chrome.automation API.
// The chrome.automation API is documented here: https://developer.chrome.com/extensions/automation
package uiauto

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/checked"
	"chromiumos/tast/local/chrome/uiauto/event"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/restriction"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/state"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

// Context is the context used when interacting with chrome.automation.
// Each individual UI interaction is limited by the pollOpts such that it will return an error when the pollOpts timeout.
type Context struct {
	tconn    *chrome.TestConn
	pollOpts testing.PollOptions
}

// New returns an Context that uses tconn to communicate to chrome.automation.
// It sets the poll options to the default interval and timeout.
func New(tconn *chrome.TestConn) *Context {
	return &Context{
		tconn: tconn,
		pollOpts: testing.PollOptions{
			Interval: 300 * time.Millisecond,
			Timeout:  15 * time.Second,
		},
	}
}

// WithTimeout returns a new Context with the specified timeout.
func (ac *Context) WithTimeout(timeout time.Duration) *Context {
	return &Context{
		tconn: ac.tconn,
		pollOpts: testing.PollOptions{
			Interval: ac.pollOpts.Interval,
			Timeout:  timeout,
		},
	}
}

// WithInterval returns a new Context with the specified polling interval.
func (ac *Context) WithInterval(interval time.Duration) *Context {
	return &Context{
		tconn: ac.tconn,
		pollOpts: testing.PollOptions{
			Interval: interval,
			Timeout:  ac.pollOpts.Timeout,
		},
	}
}

// WithPollOpts returns a new Context with the specified polling options.
func (ac *Context) WithPollOpts(pollOpts testing.PollOptions) *Context {
	return &Context{
		tconn:    ac.tconn,
		pollOpts: pollOpts,
	}
}

// Action is a function that takes a context and returns an error.
type Action = func(context.Context) error

// NamedAction gives a name to an action. It logs when an action starts,
// and if the action fails, tells you the name of the failing action.
func NamedAction(name string, fn Action) Action {
	return func(ctx context.Context) error {
		if err := fn(ctx); err != nil {
			return errors.Wrapf(err, "failed action %s", name)
		}
		return nil
	}
}

// Combine combines a list of functions from Context to error into one function.
// Combine adds the name of the operation into the error message to clarify the step.
// It is recommended to start the name of operations with a verb, e.g.,
//     "open Downloads and right click a folder"
// Then the failure msg would be like:
//     "failed to open Downloads and right click a folder on step ..."
func Combine(name string, steps ...Action) Action {
	return func(ctx context.Context) error {
		for i, f := range steps {
			if err := f(ctx); err != nil {
				return errors.Wrapf(err, "failed to %s on step %d", name, i+1)
			}
		}
		return nil
	}
}

// Retry returns a function that retries a given action if it returns error.
// The action will be executed up to n times, including the first attempt.
// The last error will be returned.  Any other errors will be silently logged.
func Retry(n int, action Action) Action {
	return func(ctx context.Context) error {
		var err error
		for i := 0; i < n; i++ {
			if err = action(ctx); err == nil {
				return nil
			}
			testing.ContextLogf(ctx, "Retry failed attempt %d: %v", i+1, err)
		}
		return err
	}
}

// NodeInfo is a mapping of chrome.automation API AutomationNode.
// It is used to get information about a specific node from JS to Go.
// NodeInfo intentionally leaves out many properties. If they become needed, add them to the Node struct.
// As defined in chromium/src/extensions/common/api/automation.idl
// Exported fields are sorted in alphabetical order.
type NodeInfo struct {
	Checked        checked.Checked         `json:"checked,omitempty"`
	ClassName      string                  `json:"className,omitempty"`
	HTMLAttributes map[string]string       `json:"htmlAttributes,omitempty"`
	Location       coords.Rect             `json:"location,omitempty"`
	Name           string                  `json:"name,omitempty"`
	Restriction    restriction.Restriction `json:"restriction,omitempty"`
	Role           role.Role               `json:"role,omitempty"`
	State          map[state.State]bool    `json:"state,omitempty"`
	Value          string                  `json:"value,omitempty"`
}

// Info returns the information for the node found by the input finder.
func (ac *Context) Info(ctx context.Context, finder *nodewith.Finder) (*NodeInfo, error) {
	q, err := finder.GenerateQuery()
	if err != nil {
		return nil, err
	}
	query := fmt.Sprintf(`
		(async () => {
			%s
			return {
				checked: node.checked,
				className: node.className,
				htmlAttributes: node.htmlAttributes,
				location: node.location,
				name: node.name,
				restriction: node.restriction,
				role: node.role,
				state: node.state,
				value: node.value,
			}
		})()
	`, q)
	var out NodeInfo
	err = testing.Poll(ctx, func(ctx context.Context) error {
		return ac.tconn.Eval(ctx, query, &out)
	}, &ac.pollOpts)
	return &out, err
}

// NodesInfo returns an array of the information for the nodes found by the input finder.
// Note that the returning array might not contain any node.
func (ac *Context) NodesInfo(ctx context.Context, finder *nodewith.Finder) ([]NodeInfo, error) {
	q, err := finder.GenerateQueryForMultipleNodes()
	if err != nil {
		return nil, err
	}
	query := fmt.Sprintf(`
		(async () => {
			%s
			var result = [];
			nodes.forEach(function(nd) {
				result.push({
					checked: nd.checked,
				    	className: nd.className,
				    	htmlAttributes: nd.htmlAttributes,
				    	location: nd.location,
				    	name: nd.name,
				    	restriction: nd.restriction,
				    	role: nd.role,
				    	state: nd.state,
				    	value: nd.value,
				});
			});
			return result
		})()
	`, q)
	var out []NodeInfo
	err = testing.Poll(ctx, func(ctx context.Context) error {
		return ac.tconn.Eval(ctx, query, &out)
	}, &ac.pollOpts)
	return out, err
}

// Location returns the location of the node found by the input finder.
// It will wait until the location is the same for a two iterations of polling.
func (ac *Context) Location(ctx context.Context, finder *nodewith.Finder) (*coords.Rect, error) {
	q, err := finder.GenerateQuery()
	if err != nil {
		return nil, err
	}
	query := fmt.Sprintf(`
		(async () => {
			%s
			return node.location;
		})()
	`, q)
	var lastLocation coords.Rect
	var currentLocation coords.Rect
	start := time.Now()
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := ac.tconn.Eval(ctx, query, &currentLocation); err != nil {
			// Reset lastLocation on error.
			lastLocation = coords.Rect{}
			return err
		}
		if currentLocation != lastLocation {
			lastLocation = currentLocation
			elapsed := time.Since(start)
			return errors.Errorf("node has not stopped changing location after %s, perhaps increase timeout or use ImmediateLocation", elapsed)
		}
		return nil
	}, &ac.pollOpts); err != nil {
		return nil, err
	}
	return &currentLocation, nil
}

// ImmediateLocation returns the location of the node found by the input finder.
// It will not wait for the location to be stable.
func (ac *Context) ImmediateLocation(ctx context.Context, finder *nodewith.Finder) (*coords.Rect, error) {
	q, err := finder.GenerateQuery()
	if err != nil {
		return nil, err
	}
	query := fmt.Sprintf(`
		(async () => {
			%s
			return node.location;
		})()
	`, q)
	var loc coords.Rect
	if err := ac.tconn.Eval(ctx, query, &loc); err != nil {
		return nil, err
	}
	return &loc, nil
}

// WaitForLocation returns a function that waits until the node location is
// stabilized.
func (ac *Context) WaitForLocation(finder *nodewith.Finder) Action {
	return func(ctx context.Context) error {
		// Invokes Location method to wait for the location to be stabilized.
		_, err := ac.Location(ctx, finder)
		return err
	}
}

// Exists returns a function that returns nil if a node exists.
// If any node in the chain is not found, it will return an error.
func (ac *Context) Exists(finder *nodewith.Finder) Action {
	return func(ctx context.Context) error {
		q, err := finder.GenerateQuery()
		if err != nil {
			return err
		}
		query := fmt.Sprintf(`
		(async () => {
			%s
			return !!node;
		})()
	`, q)
		var exists bool
		if err := ac.tconn.Eval(ctx, query, &exists); err != nil {
			return err
		}
		if !exists {
			return errors.New("node does not exist")
		}
		return nil
	}
}

// WaitUntilExists returns a function that waits until the node found by the input finder exists.
func (ac *Context) WaitUntilExists(finder *nodewith.Finder) Action {
	return func(ctx context.Context) error {
		return testing.Poll(ctx, func(ctx context.Context) error {
			return ac.Exists(finder)(ctx)
		}, &ac.pollOpts)
	}
}

// EnsureGoneFor returns a function that check the specified node does not exist for the timeout period.
func (ac *Context) EnsureGoneFor(finder *nodewith.Finder, duration time.Duration) Action {
	return func(ctx context.Context) error {
		return testing.Poll(ctx, func(ctx context.Context) error {
			return ac.Gone(finder)(ctx)
		}, &testing.PollOptions{Timeout: duration})
	}
}

// Gone returns a function that returns nil if a node does not exist.
// If any node in the chain is not found, it will return nil.
func (ac *Context) Gone(finder *nodewith.Finder) Action {
	return func(ctx context.Context) error {
		q, err := finder.GenerateQuery()
		if err != nil {
			return err
		}
		query := fmt.Sprintf(`
		(async () => {
			%s
			return !!node;
		})()
	`, q)
		var exists bool
		if err := ac.tconn.Eval(ctx, query, &exists); err != nil {
			// Only consider the node gone if we get a not found error.
			if strings.Contains(err.Error(), nodewith.ErrNotFound) {
				return nil
			}
			return err
		}
		if exists {
			return errors.New("node still exists")
		}
		return nil
	}
}

// WaitUntilGone returns a function that waits until the node found by the input finder is gone.
func (ac *Context) WaitUntilGone(finder *nodewith.Finder) Action {
	return func(ctx context.Context) error {
		return testing.Poll(ctx, func(ctx context.Context) error {
			return ac.Gone(finder)(ctx)
		}, &ac.pollOpts)
	}
}

// clickType describes how user clicks mouse.
type clickType int

const (
	leftClick clickType = iota
	rightClick
	doubleClick
)

// mouseClick returns a function that clicks on the location of the node found by the input finder.
// It will wait until the location is stable before clicking.
// This returns a function to make it chainable in ui.Run.
func (ac *Context) mouseClick(ct clickType, finder *nodewith.Finder) Action {
	return func(ctx context.Context) error {
		loc, err := ac.Location(ctx, finder)
		if err != nil {
			return err
		}
		switch ct {
		case leftClick:
			return mouse.Click(ac.tconn, loc.CenterPoint(), mouse.LeftButton)(ctx)
		case rightClick:
			return mouse.Click(ac.tconn, loc.CenterPoint(), mouse.RightButton)(ctx)
		case doubleClick:
			return mouse.DoubleClick(ac.tconn, loc.CenterPoint(), 100*time.Millisecond)(ctx)
		default:
			return errors.New("invalid click type")
		}
	}
}

// MouseClickAtLocation returns a function that clicks on the specified location.
// This returns a function to make it chainable in ui.Run.
func (ac *Context) MouseClickAtLocation(ct clickType, loc coords.Point) Action {
	switch ct {
	case leftClick:
		return mouse.Click(ac.tconn, loc, mouse.LeftButton)
	case rightClick:
		return mouse.Click(ac.tconn, loc, mouse.RightButton)
	case doubleClick:
		return mouse.DoubleClick(ac.tconn, loc, 100*time.Millisecond)
	default:
		return func(ctx context.Context) error {
			return errors.New("invalid click type")
		}
	}
}

// immediateMouseClick returns a function that clicks on the location of the node found by the input finder.
// It will not wait until the location is stable before clicking.
// This returns a function to make it chainable in ui.Run.
func (ac *Context) immediateMouseClick(ct clickType, finder *nodewith.Finder) Action {
	return func(ctx context.Context) error {
		loc, err := ac.ImmediateLocation(ctx, finder)
		if err != nil {
			return err
		}
		switch ct {
		case leftClick:
			return mouse.Click(ac.tconn, loc.CenterPoint(), mouse.LeftButton)(ctx)
		case rightClick:
			return mouse.Click(ac.tconn, loc.CenterPoint(), mouse.RightButton)(ctx)
		case doubleClick:
			return mouse.DoubleClick(ac.tconn, loc.CenterPoint(), 100*time.Millisecond)(ctx)
		default:
			return errors.New("invalid click type")
		}
	}
}

// LeftClick returns a function that left clicks on the location of the node found by the input finder.
// It will wait until the location is stable before clicking.
// This returns a function to make it chainable in ui.Run.
func (ac *Context) LeftClick(finder *nodewith.Finder) Action {
	return ac.mouseClick(leftClick, finder)
}

// RightClick returns a function that right clicks on the location of the node found by the input finder.
// It will wait until the location is stable before clicking.
// This returns a function to make it chainable in ui.Run.
func (ac *Context) RightClick(finder *nodewith.Finder) Action {
	return ac.mouseClick(rightClick, finder)
}

// DoubleClick returns a function that double clicks on the location of the node found by the input finder.
// It will wait until the location is stable before clicking.
// This returns a function to make it chainable in ui.Run.
func (ac *Context) DoubleClick(finder *nodewith.Finder) Action {
	return ac.mouseClick(doubleClick, finder)
}

// ImmediateLeftClick returns a function that left clicks on the location of the node found by the input finder.
// It will not wait until the location is stable before clicking.
// This returns a function to make it chainable in ui.Run.
func (ac *Context) ImmediateLeftClick(finder *nodewith.Finder) Action {
	return ac.immediateMouseClick(leftClick, finder)
}

// ImmediateRightClick returns a function that right clicks on the location of the node found by the input finder.
// It will not wait until the location is stable before clicking.
// This returns a function to make it chainable in ui.Run.
func (ac *Context) ImmediateRightClick(finder *nodewith.Finder) Action {
	return ac.immediateMouseClick(rightClick, finder)
}

// ImmediateDoubleClick returns a function that double clicks on the location of the node found by the input finder.
// It will not wait until the location is stable before clicking.
// This returns a function to make it chainable in ui.Run.
func (ac *Context) ImmediateDoubleClick(finder *nodewith.Finder) Action {
	return ac.immediateMouseClick(doubleClick, finder)
}

// LeftClickUntil returns a function that repeatedly left clicks the node until the condition returns no error.
// It will try to click the node once before it checks the condition.
// This is useful for situations where there is no indication of whether the node is ready to receive clicks.
// It uses the polling options from the Context.
func (ac *Context) LeftClickUntil(finder *nodewith.Finder, condition func(context.Context) error) Action {
	return func(ctx context.Context) error {
		if err := ac.LeftClick(finder)(ctx); err != nil {
			return errors.Wrap(err, "failed to initially click the node")
		}
		if err := testing.Sleep(ctx, ac.pollOpts.Interval); err != nil {
			return err
		}
		return testing.Poll(ctx, func(ctx context.Context) error {
			if err := condition(ctx); err != nil {
				if err := ac.ImmediateLeftClick(finder)(ctx); err != nil {
					return errors.Wrap(err, "failed to click the node")
				}
				return errors.Wrap(err, "click may not have been received yet")
			}
			return nil
		}, &ac.pollOpts)
	}
}

// FocusAndWait returns a function that calls the focus() JS method of the found node.
// This can be used to scroll to nodes which aren't currently visible, enabling them to be clicked.
// The focus event is not instant, so an EventWatcher (watcher.go) is used to check its status.
// The EventWatcher waits the duration of timeout for the event to occur.
func (ac *Context) FocusAndWait(finder *nodewith.Finder) Action {
	return func(ctx context.Context) error {
		ew, err := NewRootWatcher(ctx, ac.tconn, event.Focus)
		if err != nil {
			return errors.Wrap(err, "failed to create focus event watcher")
		}
		defer ew.Release(ctx)

		q, err := finder.GenerateQuery()
		if err != nil {
			return err
		}
		query := fmt.Sprintf(`
		(async () => {
			%s
			node.focus();
		})()
	`, q)

		return testing.Poll(ctx, func(ctx context.Context) error {
			if err := ac.tconn.Eval(ctx, query, nil); err != nil {
				return errors.Wrap(err, "failed to call focus() on the node")
			}

			if _, err := ew.WaitForEvent(ctx, ac.pollOpts.Timeout); err != nil {
				return errors.Wrap(err, "failed to wait for the focus event on the specified node")
			}
			return nil
		}, &ac.pollOpts)
	}
}

// MakeVisible returns a function that calls makeVisible() JS method to make found node visible.
func (ac *Context) MakeVisible(finder *nodewith.Finder) Action {
	return func(ctx context.Context) error {
		q, err := finder.GenerateQuery()
		if err != nil {
			return err
		}
		query := fmt.Sprintf(`
		(async () => {
			%s
			node.makeVisible();
		})()
	`, q)

		if err := ac.tconn.Eval(ctx, query, nil); err != nil {
			return errors.Wrap(err, "failed to call makeVisible() on the node")
		}
		return nil
	}
}

// IfSuccessThen returns a function that runs action only if the first function succeeds.
// The function returns an error only if the preFunc succeeds but action fails,
// It returns nil in all other situations.
// Example:
//   dialog := nodewith.Name("Dialog").Role(role.Dialog)
//   button := nodewith.Name("Ok").Role(role.Button).Ancestor(dialog)
//   ui := uiauto.New(tconn)
//   if err := ui.IfSuccessThen(ui.Withtimeout(5*time.Second).WaitUntilExists(dialog), ui.LeftClick(button))(ctx); err != nil {
//	    ...
//   }
func (ac *Context) IfSuccessThen(preFunc func(context.Context) error, action Action) Action {
	return func(ctx context.Context) error {
		if err := preFunc(ctx); err == nil {
			if err := action(ctx); err != nil {
				return err
			}
		} else {
			testing.ContextLogf(ctx, "The prefunc failed, the action was not executed, the error was: %s", err)
		}
		return nil
	}
}

// Retry returns a function that retries a given action if it returns error.
// The action will be executed up to n times, including the first attempt.
// The last error will be returned.  Any other errors will be silently logged.
// Between each run of the loop, it will sleep according the the uiauto.Context pollOpts.
func (ac *Context) Retry(n int, action Action) Action {
	return func(ctx context.Context) error {
		var err error
		for i := 0; i < n; i++ {
			if err = action(ctx); err == nil {
				return nil
			}
			testing.ContextLogf(ctx, "Retry failed attempt %d: %v", i+1, err)
			// Sleep between all iterations.
			if i < n-1 {
				if err := testing.Sleep(ctx, ac.pollOpts.Interval); err != nil {
					testing.ContextLog(ctx, "Failed to sleep between retry iterations: ", err)
				}
			}
		}
		return err
	}
}
