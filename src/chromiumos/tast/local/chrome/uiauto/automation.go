// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package uiauto enables automating with the ChromeOS UI through the chrome.automation API.
// The chrome.automation API is documented here: https://developer.chrome.com/extensions/automation
package uiauto

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/checked"
	"chromiumos/tast/local/chrome/uiauto/event"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/restriction"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/state"
	"chromiumos/tast/local/chrome/useractions"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

// NodeInfoJSFunc is a string of JavaScript code defining a function getNodeInfo.
// getNodeInfo returns an object in the shape of NodeInfo.
const NodeInfoJSFunc = `var getNodeInfo = function(node) {
	var info = {
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
	if (node.nextSibling) {
		info.nextSibling = getNodeInfo(node.nextSibling);
	}
	return info;
}`

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
type Action = action.Action

// NamedAction gives a name to an action. It logs when an action starts,
// and if the action fails, tells you the name of the failing action.
func NamedAction(name string, fn Action) Action {
	return action.Named(name, fn)
}

// UserAction wraps an action with context information.
// For more details, refer to https://source.chromium.org/chromiumos/chromiumos/codesearch/+/main:src/platform/tast-tests/src/chromiumos/tast/local/chrome/useractions/README.md.
func UserAction(name string, fn Action, uc *useractions.UserContext, cfg *useractions.UserActionCfg) Action {
	userAction := useractions.NewUserAction(name, fn, uc, cfg)
	return userAction.Run
}

// Combine combines a list of functions from Context to error into one function.
// Combine adds the name of the operation into the error message to clarify the step.
// It is recommended to start the name of operations with a verb, e.g.,
//     "open Downloads and right click a folder"
// Then the failure msg would be like:
//     "failed to open Downloads and right click a folder on step ..."
func Combine(name string, steps ...Action) Action {
	return action.Combine(name, steps...)
}

// Retry returns a function that retries a given action if it returns error.
// The action will be executed up to n times, including the first attempt.
// The last error will be returned.  Any other errors will be silently logged.
func Retry(n int, fn Action) Action {
	return action.Retry(n, fn, 0)
}

// RetrySilently returns a function that retries a given action if it returns error.
// The action will be executed up to n times, including the first attempt.
// The last error will be returned.  Any other errors will be ignored.
func RetrySilently(n int, fn Action) Action {
	return action.RetrySilently(n, fn, 0)
}

// Repeat returns a function that runs the specified function repeatedly for the specific number of times.
func Repeat(n int, fn Action) Action {
	return func(ctx context.Context) error {
		for i := 0; i < n; i++ {
			if err := fn(ctx); err != nil {
				return err
			}
		}
		return nil
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
	NextSibling    *NodeInfo               `json:"nextSibling,omitempty"`
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
			%s
			return getNodeInfo(node);
		})()
	`, q, NodeInfoJSFunc)
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
			%s
			var result = [];
			nodes.forEach(function(node) {
				result.push(getNodeInfo(node));
			});
			return result
		})()
	`, q, NodeInfoJSFunc)
	var out []NodeInfo
	err = testing.Poll(ctx, func(ctx context.Context) error {
		return ac.tconn.Eval(ctx, query, &out)
	}, &ac.pollOpts)
	return out, err
}

// Matches returns whether |finder| matches |actual|. Another way of
// saying this is "does |finder| map to |actual|?" or "are the properties listed
// in |finder| present in |actual|?".
func (ac *Context) Matches(ctx context.Context, finder *nodewith.Finder, actual *NodeInfo) (bool, error) {
	candidate, err := ac.Info(ctx, finder)
	if err != nil {
		return false, errors.Wrap(err, "failed to find NodeInfo")
	}

	return reflect.DeepEqual(*candidate, *actual), nil
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

// WaitForEvent returns a function that conducts the specified action, and
// then waits for the specified event appears on the specified node. It takes
// an action as an argument rather than it is a part of a chain of action
// because it needs to set up a watcher in prior to the action, and also
// it needs to clean up the allocated resources for the watcher afterwards.
func (ac *Context) WaitForEvent(finder *nodewith.Finder, ev event.Event, act Action) Action {
	return func(ctx context.Context) error {
		watcher, err := ac.setupWatcher(ctx, finder, ev)
		if err != nil {
			return err
		}
		defer watcher.release(ctx)

		if err := act(ctx); err != nil {
			return errors.Wrap(err, "failed to run the main action")
		}
		return testing.Poll(ctx, func(ctx context.Context) error {
			var events []map[string]interface{}
			if err := watcher.Call(ctx, &events, `function() { return this.events; }`); err != nil {
				return testing.PollBreak(err)
			}
			if len(events) == 0 {
				return errors.New("events haven't occurred yet")
			}
			return nil
		}, &ac.pollOpts)
	}
}

// WaitUntilNoEvent returns a function that waits until a specified event has stopped
// to appear for the specified node.
func (ac *Context) WaitUntilNoEvent(finder *nodewith.Finder, ev event.Event) Action {
	return func(ctx context.Context) error {
		watcher, err := ac.setupWatcher(ctx, finder, ev)
		if err != nil {
			return err
		}
		defer watcher.release(ctx)

		previousEventCount := -1
		var currentEventCount int

		return testing.Poll(ctx, func(ctx context.Context) error {
			var events []map[string]interface{}
			if err := watcher.Call(ctx, &events, `function() { return this.events; }`); err != nil {
				return testing.PollBreak(err)
			}

			// When the event counts are the same between two subsequent polls, events are considered to
			// have stopped appearing.
			currentEventCount = len(events)
			if previousEventCount == currentEventCount {
				return nil
			}
			previousEventCount = currentEventCount
			return errors.New("received new events between polls")
		}, &ac.pollOpts)
	}
}

// watcher is used for interfacing with the EventListener of chrome.AutomationNode to
// listen for events.
type watcher struct {
	*chrome.JSObject
}

// setupWatcher sets up a watcher for the specified event type on the specified node.
// watcher.release(ctx) is needed to clean up the allocated resources for the watcher afterwards.
func (ac *Context) setupWatcher(ctx context.Context, finder *nodewith.Finder, ev event.Event) (*watcher, error) {
	q, err := finder.GenerateQuery()
	if err != nil {
		return nil, err
	}
	expr := fmt.Sprintf(`async function(eventType) {
		%s
		let watcher = {
			"events": [],
			"callback": (ev) => {
				watcher.events.push(ev);
			},
			"release": () => {
				node.removeEventListener(eventType, watcher.callback);
			}
		};
		node.addEventListener(eventType, watcher.callback);
		return watcher;
	}`, q)

	obj := &chrome.JSObject{}
	if err := ac.tconn.Call(ctx, obj, expr, ev); err != nil {
		return nil, errors.Wrap(err, "failed to execute the registration")
	}
	return &watcher{obj}, nil
}

// release cleans up the allocated resources for the watcher.
func (w *watcher) release(ctx context.Context) {
	w.Release(ctx)
	w.Call(ctx, nil, `function() { this.release(); }`)
}

// Select sets the document selection to include everything between the two nodes at the offsets.
func (ac *Context) Select(startNodeFinder *nodewith.Finder, startOffset int, endNodeFinder *nodewith.Finder, endOffset int) Action {
	return func(ctx context.Context) error {
		qStart, err := startNodeFinder.GenerateQuery()
		if err != nil {
			return err
		}

		qEnd, err := endNodeFinder.GenerateQuery()
		if err != nil {
			return err
		}

		// Use the nodeFinder code generation to get the start and end nodes.
		// The statements are enclosed in block to avoid naming collision.
		query := fmt.Sprintf(`
		(async () => {
			let startNode;
			let endNode;
			{
				%s
				startNode = node;
			}
			{
				%s
				endNode = node;
			}
			chrome.automation.setDocumentSelection({
				anchorObject: startNode,
				anchorOffset: %d,
				focusObject: endNode,
				focusOffset: %d
			  });
		})()
		`, qStart, qEnd, startOffset, endOffset)

		return ac.tconn.Eval(ctx, query, nil)
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
		})()
	`, q)
		return ac.tconn.Eval(ctx, query, nil)
	}
}

// IsNodeFound immediately checks if any nodes found with given finder.
// It returns true if found otherwise false.
func (ac *Context) IsNodeFound(ctx context.Context, finder *nodewith.Finder) (bool, error) {
	if err := ac.Exists(finder)(ctx); err != nil {
		if strings.Contains(err.Error(), nodewith.ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// WaitUntilExists returns a function that waits until the node found by the input finder exists.
func (ac *Context) WaitUntilExists(finder *nodewith.Finder) Action {
	return func(ctx context.Context) error {
		return testing.Poll(ctx, ac.Exists(finder), &ac.pollOpts)
	}
}

// ErrNodeAppeared is returned if node is expected not to be visible
var ErrNodeAppeared = errors.New("node appeared when it should not")

// EnsureGoneFor returns a function that check the specified node does not
// exist for the timeout period. Notice the usage of this function in your
// code:
// 1. If you expect an ui-node to go away and not to appear again use
// WaitUntilGone succeeded with EnsureGoneFor.
// 2. If you expect an ui-node not to appear at all use EnsureGoneFor.
func (ac *Context) EnsureGoneFor(finder *nodewith.Finder, duration time.Duration) Action {
	return func(ctx context.Context) error {
		// Use custom timeout watchdog rather than relying on context due to
		// possible race condition. More context is here https://groups.google.com/a/google.com/g/tast-reviewers/c/sGxqggEGVAg/
		start := time.Now()
		return testing.Poll(ctx,
			func(ctx context.Context) error {
				if err := ac.Exists(finder)(ctx); err == nil {
					// If node exists break the poll immediately with error.
					return testing.PollBreak(ErrNodeAppeared)
				}
				if time.Since(start) >= duration {
					// Timeout is reached and element was not found.
					return nil
				}
				return errors.Errorf("still waiting for the node for %.1fs", (duration - time.Since(start)).Seconds())
			},
			nil,
		)
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
		return testing.Poll(ctx, ac.Gone(finder), &ac.pollOpts)
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

// RightClickUntil returns a function that repeatedly right clicks the node until the condition returns no error.
// It will try to click the node once before it checks the condition.
// This is useful for situations where there is no indication of whether the node is ready to receive clicks.
// It uses the polling options from the Context.
func (ac *Context) RightClickUntil(finder *nodewith.Finder, condition func(context.Context) error) Action {
	return func(ctx context.Context) error {
		if err := ac.RightClick(finder)(ctx); err != nil {
			return errors.Wrap(err, "failed to initially click the node")
		}
		if err := testing.Sleep(ctx, ac.pollOpts.Interval); err != nil {
			return err
		}
		return testing.Poll(ctx, func(ctx context.Context) error {
			if err := condition(ctx); err != nil {
				if err := ac.ImmediateRightClick(finder)(ctx); err != nil {
					return errors.Wrap(err, "failed to click the node")
				}
				return errors.Wrap(err, "click may not have been received yet")
			}
			return nil
		}, &ac.pollOpts)
	}
}

// RetryUntil returns a function that repeatedly does the given action until the condition returns no error.
// It will try to do action once before it checks the condition.
// It uses the polling options from the Context.
func (ac *Context) RetryUntil(action, condition Action) Action {
	return func(ctx context.Context) error {
		if err := action(ctx); err != nil {
			return errors.Wrap(err, "failed to initially do action")
		}
		if err := testing.Sleep(ctx, ac.pollOpts.Interval); err != nil {
			return err
		}
		return testing.Poll(ctx, func(ctx context.Context) error {
			if err := condition(ctx); err != nil {
				if err := action(ctx); err != nil {
					return errors.Wrap(err, "failed to do action")
				}
				return errors.Wrap(err, "action has been done but condition is not met")
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
	return ac.WaitForEvent(nodewith.Root(), event.Focus, func(ctx context.Context) error {
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
			return ac.tconn.Eval(ctx, query, nil)
		}, &ac.pollOpts)
	})
}

// EnsureFocused returns a function that ensures the found node is focused.
// This can be used to focus on nodes whose state isn't certained.
// It checks the found node's state and calls FocusAndWait() only if the node is not focused.
func (ac *Context) EnsureFocused(finder *nodewith.Finder) Action {
	return func(ctx context.Context) error {
		info, err := ac.Info(ctx, finder)
		if err != nil {
			return err
		}
		if info.State[state.Focused] {
			return nil
		}

		if err = ac.FocusAndWait(finder)(ctx); err != nil {
			// There are chances that the focus event does not occur but the node is already focused.
			return ac.WaitUntilExists(finder.Focused())(ctx)
		}
		return nil
	}
}

// MouseMoveTo returns a function moving the mouse to hover on the center point of located node.
// When duration is 0, it moves instantly to the specified location.
// Otherwise, the cursor should move linearly during the period.
// Unlike mouse.Move which is designed to move to a fixed location,
// this function moves to the target location immediately after getting it,
// avoid the need of getting it in advance.
// It addresses the cases that the node only becomes available
// or changes location in the middle of a sequence of combined steps.
func (ac *Context) MouseMoveTo(finder *nodewith.Finder, duration time.Duration) Action {
	return func(ctx context.Context) error {
		location, err := ac.Location(ctx, finder)
		if err != nil {
			return errors.Wrapf(err, "failed to get location of %v", finder)
		}
		return mouse.Move(ac.tconn, location.CenterPoint(), duration)(ctx)
	}
}

// Sleep returns a function sleeping given time duration.
func Sleep(d time.Duration) Action {
	return func(ctx context.Context) error {
		return testing.Sleep(ctx, d)
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
//   if err := uiauto.IfSuccessThen(ui.WithTimeout(5*time.Second).WaitUntilExists(dialog), ui.LeftClick(button))(ctx); err != nil {
//	    ...
//   }
func IfSuccessThen(preFunc, fn Action) Action {
	return action.IfSuccessThen(preFunc, fn)
}

// Retry returns a function that retries a given action if it returns error.
// The action will be executed up to n times, including the first attempt.
// The last error will be returned.  Any other errors will be silently logged.
// Between each run of the loop, it will sleep according the the uiauto.Context pollOpts.
func (ac *Context) Retry(n int, fn Action) Action {
	return action.Retry(n, fn, ac.pollOpts.Interval)
}

// RetrySilently returns a function that retries a given action if it returns error.
// The action will be executed up to n times, including the first attempt.
// The last error will be returned.  Any other errors will be ignored.
// Between each run of the loop, it will sleep according the the uiauto.Context pollOpts.
func (ac *Context) RetrySilently(n int, fn Action) Action {
	return action.RetrySilently(n, fn, ac.pollOpts.Interval)
}

// CheckRestriction returns a function that checks the restriction of the node found by the input finder is as expected.
// disabled/enabled is a common usecase, e.g,
//    CheckRestriction(installButton, restriction.Disabled)
//    CheckRestriction(installButton, restriction.None)
func (ac *Context) CheckRestriction(finder *nodewith.Finder, restriction restriction.Restriction) Action {
	return func(ctx context.Context) error {
		nodeInfo, err := ac.Info(ctx, finder)
		if err != nil {
			return err
		}
		if nodeInfo.Restriction != restriction {
			return errors.Wrapf(err, "failed to check restriction state: got %v, want %v", nodeInfo.Restriction, restriction)
		}
		return nil
	}
}

// DoDefault returns a function that calls doDefault() JS method to trigger the
// default action on a node regardless of its location, e.g. left click on a button.
// This function can be used when the a11y tree fails to find the accurate location
// of a node thus mouse.LeftClick() fails consequently.
func (ac *Context) DoDefault(finder *nodewith.Finder) Action {
	return func(ctx context.Context) error {
		q, err := finder.GenerateQuery()
		if err != nil {
			return err
		}
		query := fmt.Sprintf(`
		(async () => {
			%s
			node.doDefault();
		})()
	`, q)

		if err := testing.Poll(ctx, func(ctx context.Context) error {
			return ac.tconn.Eval(ctx, query, nil)
		}, &ac.pollOpts); err != nil {
			return errors.Wrap(err, "failed to call doDefault() on the node")
		}

		return nil
	}
}

// ResetScrollOffset returns a function that calls setScrollOffset(0, 0) JS method to reset the
// scroll offset on a node to scroll it to its default scroll position.
func (ac *Context) ResetScrollOffset(finder *nodewith.Finder) Action {
	return func(ctx context.Context) error {
		q, err := finder.GenerateQuery()
		if err != nil {
			return err
		}
		query := fmt.Sprintf(`
		(async () => {
			%s
			node.setScrollOffset(0, 0);
		})()
	`, q)

		if err := testing.Poll(ctx, func(ctx context.Context) error {
			return ac.tconn.Eval(ctx, query, nil)
		}, &ac.pollOpts); err != nil {
			return errors.Wrap(err, "failed to call setScrollOffset() on the node")
		}

		return nil
	}

}
