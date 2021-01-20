// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ui2 enables interacting with the ChromeOS UI through the chrome.automation API.
// The chrome.automation API is documented here: https://developer.chrome.com/extensions/automation
package ui2

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui/mouse"
	"chromiumos/tast/local/chrome/ui2/checked"
	"chromiumos/tast/local/chrome/ui2/nodewith"
	"chromiumos/tast/local/chrome/ui2/restriction"
	"chromiumos/tast/local/chrome/ui2/role"
	"chromiumos/tast/local/chrome/ui2/state"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

// AutomationContext is the context used when interacting with chrome.automation.
type AutomationContext struct {
	tconn    *chrome.TestConn
	pollOpts testing.PollOptions
}

// New returns an AutomationContext that uses tconn to communicate to chrome.automation.
// It sets the poll options to the default interval and timeout.
func New(tconn *chrome.TestConn) *AutomationContext {
	return &AutomationContext{
		tconn: tconn,
		pollOpts: testing.PollOptions{
			Interval: 300 * time.Millisecond,
			Timeout:  30 * time.Second,
		},
	}
}

// WithTimeout returns a new AutomationContext with the specified timeout.
func (ac *AutomationContext) WithTimeout(timeout time.Duration) *AutomationContext {
	return &AutomationContext{
		tconn: ac.tconn,
		pollOpts: testing.PollOptions{
			Interval: ac.pollOpts.Interval,
			Timeout:  timeout,
		},
	}
}

// WithInterval returns a new AutomationContext with the specified polling interval.
func (ac *AutomationContext) WithInterval(interval time.Duration) *AutomationContext {
	return &AutomationContext{
		tconn: ac.tconn,
		pollOpts: testing.PollOptions{
			Interval: interval,
			Timeout:  ac.pollOpts.Timeout,
		},
	}
}

// WithPollOpts returns a new AutomationContext with the specified polling options.
func (ac *AutomationContext) WithPollOpts(pollOpts testing.PollOptions) *AutomationContext {
	return &AutomationContext{
		tconn:    ac.tconn,
		pollOpts: pollOpts,
	}
}

func (ac *AutomationContext) findQuery(finders ...*nodewith.Finder) (string, error) {
	if len(finders) == 0 {
		return "", errors.New("at least one nodewith.Finder must be specified")
	}
	out := `
		let node = await tast.promisify(chrome.automation.getDesktop)();
		let nodes = [];
	`
	for _, f := range finders {
		q, err := f.FindQuery()
		if err != nil {
			return "", err
		}
		out += q
	}
	return out, nil
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

// Info returns the information for the node found by the input finders.
func (ac *AutomationContext) Info(ctx context.Context, finders ...*nodewith.Finder) (*NodeInfo, error) {
	q, err := ac.findQuery(finders...)
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

// Location returns the location of the node found by the input finders.
// It will wait until the location is the same for a two iterations of polling.
func (ac *AutomationContext) Location(ctx context.Context, finders ...*nodewith.Finder) (*coords.Rect, error) {
	q, err := ac.findQuery(finders...)
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
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := ac.tconn.Eval(ctx, query, &currentLocation); err != nil {
			// Reset lastLocation on error.
			lastLocation = coords.Rect{}
			return err
		}
		if currentLocation != lastLocation {
			lastLocation = currentLocation
			return errors.New("node location still changing")
		}
		return nil
	}, &ac.pollOpts); err != nil {
		return nil, err
	}
	return &currentLocation, nil
}

// clickType describes how user clicks mouse.
type clickType int

const (
	leftClick clickType = iota
	rightClick
	doubleClick
)

func (ac *AutomationContext) mouseClick(ctx context.Context, ct clickType, finders ...*nodewith.Finder) error {
	loc, err := ac.Location(ctx, finders...)
	if err != nil {
		return err
	}
	switch ct {
	case leftClick:
		return mouse.Click(ctx, ac.tconn, loc.CenterPoint(), mouse.LeftButton)
	case rightClick:
		return mouse.Click(ctx, ac.tconn, loc.CenterPoint(), mouse.RightButton)
	case doubleClick:
		return mouse.DoubleClick(ctx, ac.tconn, loc.CenterPoint(), 100*time.Millisecond)
	default:
		return errors.New("invalid click type")
	}
	return nil
}

// LeftClick left clicks on the location of the node found by the input finders.
// It will wait until the location is stable before clicking.
func (ac *AutomationContext) LeftClick(ctx context.Context, finder ...*nodewith.Finder) error {
	return ac.mouseClick(ctx, leftClick, finder...)
}

// RightClick right clicks on the location of the node found by the input finders.
// It will wait until the location is stable before clicking.
func (ac *AutomationContext) RightClick(ctx context.Context, finder ...*nodewith.Finder) error {
	return ac.mouseClick(ctx, rightClick, finder...)
}

// DoubleClick double clicks on the location of the node found by the input finders.
// It will wait until the location is stable before clicking.
func (ac *AutomationContext) DoubleClick(ctx context.Context, finder ...*nodewith.Finder) error {
	return ac.mouseClick(ctx, doubleClick, finder...)
}
