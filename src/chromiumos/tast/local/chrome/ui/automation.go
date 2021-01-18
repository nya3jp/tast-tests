// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ui enables interacting with the ChromeOS UI through the chrome.automation API.
// The chrome.automation API is documented here: https://developer.chrome.com/extensions/automation
package ui

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"regexp"
	"time"

	"github.com/google/go-cmp/cmp"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui/mouse"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

// FindParams is a mapping of chrome.automation.FindParams to Golang.
// Name and ClassName allow quick access because they are common attributes.
// As defined in chromium/src/extensions/common/api/automation.idl
type FindParams struct {
	Role       RoleType
	Name       string
	ClassName  string
	Attributes map[string]interface{}
	State      map[StateType]bool
}

// rawAttributes creates a byte array of the attributes field.
// It adds the quick attributes(Name and ClassName) to it as well.
// If any attribute is defined twice, an error is returned.
// This function is for use in rawBytes.
func (params *FindParams) rawAttributes() ([]byte, error) {
	attributes := make(map[string]interface{})
	if params.Attributes != nil {
		for k, v := range params.Attributes {
			attributes[k] = v
		}
	}
	// Ensure parameters aren't passed twice.
	if params.Name != "" {
		if _, exists := attributes["name"]; exists {
			return nil, errors.New("cannot set both FindParams.Name and FindParams.Attributes['name']")
		}
		attributes["name"] = params.Name
	}
	if params.ClassName != "" {
		if _, exists := attributes["className"]; exists {
			return nil, errors.New("cannot set both FindParams.ClassName and FindParams.Attributes['className']")
		}
		attributes["className"] = params.ClassName
	}

	// Return null if empty dictionary
	if len(attributes) == 0 {
		return []byte("null"), nil
	}

	// json.Marshal can't be used because this is JavaScript code with regular expressions, not JSON.
	// TODO(bhansknecht): work with chrome.automation API maintainers to support a JSON friendly regex format.
	var buf bytes.Buffer
	buf.WriteByte('{')
	first := true
	for k, v := range attributes {
		if first {
			first = false
		} else {
			buf.WriteByte(',')
		}
		switch v := v.(type) {
		case string, RoleType, CheckedState, RestrictionState:
			fmt.Fprintf(&buf, "%q:%q", k, v)
		case int, float32, float64, bool:
			fmt.Fprintf(&buf, "%q:%v", k, v)
		case regexp.Regexp, *regexp.Regexp:
			fmt.Fprintf(&buf, `%q:/%v/`, k, v)
		default:
			return nil, errors.Errorf("FindParams does not support type(%T) for parameter(%s)", v, k)
		}
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// rawBytes converts FindParams into a JSON-like object that can contain JS Regexp Notation.
// The result will be return as a byte Array.
func (params *FindParams) rawBytes() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	rawAttributes, err := params.rawAttributes()
	if err != nil {
		return nil, err
	}
	fmt.Fprintf(&buf, `"attributes":%s,`, rawAttributes)

	if params.Role != "" {
		fmt.Fprintf(&buf, `"role":%q,`, params.Role)
	}

	state, err := json.Marshal(params.State)
	if err != nil {
		return nil, err
	}
	fmt.Fprintf(&buf, `"state":%s`, state)

	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// Node is a reference to chrome.automation API AutomationNode.
// Node intentionally leaves out many properties. If they become needed, add them to the Node struct and to the Update function.
// As defined in chromium/src/extensions/common/api/automation.idl
// Exported fields are sorted in alphabetical order.
type Node struct {
	object         *chrome.JSObject
	tconn          *chrome.TestConn
	Checked        CheckedState       `json:"checked,omitempty"`
	ClassName      string             `json:"className,omitempty"`
	HTMLAttributes map[string]string  `json:"htmlAttributes,omitempty"`
	Location       coords.Rect        `json:"location,omitempty"`
	Name           string             `json:"name,omitempty"`
	Restriction    RestrictionState   `json:"restriction,omitempty"`
	Role           RoleType           `json:"role,omitempty"`
	State          map[StateType]bool `json:"state,omitempty"`
	Value          string             `json:"value,omitempty"`
}

// NodeSlice is a slice of pointers to nodes. It is used for releaseing a group of nodes.
type NodeSlice []*Node

// Release frees the reference to Javascript for this node.
func (nodes NodeSlice) Release(ctx context.Context) {
	for _, n := range nodes {
		defer n.Release(ctx)
	}
}

// NewNode creates a new node struct and initializes its fields.
// NewNode takes ownership of obj and will release it if the node fails to initialize.
func NewNode(ctx context.Context, tconn *chrome.TestConn, obj *chrome.JSObject) (*Node, error) {
	node := &Node{
		object: obj,
		tconn:  tconn,
	}
	if err := node.Update(ctx); err != nil {
		node.Release(ctx)
		return nil, errors.Wrap(err, "failed to initialize node")
	}
	return node, nil
}

// Update reloads the fields of this node.
func (n *Node) Update(ctx context.Context) error {
	return n.object.Call(ctx, n, `function(){
		return {
			checked: this.checked,
			className: this.className,
			htmlAttributes: this.htmlAttributes,
			location: this.location,
			name: this.name,
			restriction: this.restriction,
			role: this.role,
			state: this.state,
			tooltip: this.tooltip,
			value: this.value,
			valueForRange: this.valueForRange,
		}
	}`)
}

// Release frees the reference to Javascript for this node.
func (n *Node) Release(ctx context.Context) {
	n.object.Release(ctx)
}

// LeftClick executes a left click on the node.
// If the JavaScript fails to execute, an error is returned.
func (n *Node) LeftClick(ctx context.Context) error {
	return n.mouseClick(ctx, leftClick)
}

// RightClick executes a right click on the node.
// If the JavaScript fails to execute, an error is returned.
func (n *Node) RightClick(ctx context.Context) error {
	return n.mouseClick(ctx, rightClick)
}

// DoubleClick executes 2 mouse left clicks on the node.
// If the JavaScript fails to execute, an error is returned.
func (n *Node) DoubleClick(ctx context.Context) error {
	return n.mouseClick(ctx, doubleClick)
}

// StableLeftClick waits for the location to be stable and then left clicks the node.
// The location must be stable for 1 iteration of polling (default 100ms).
func (n *Node) StableLeftClick(ctx context.Context, pollOpts *testing.PollOptions) error {
	return n.stableMouseClick(ctx, leftClick, pollOpts)
}

// StableRightClick waits for the location to be stable and then right clicks the node.
// The location must be stable for 1 iteration of polling (default 100ms).
func (n *Node) StableRightClick(ctx context.Context, pollOpts *testing.PollOptions) error {
	return n.stableMouseClick(ctx, rightClick, pollOpts)
}

// StableDoubleClick waits for the location to be stable and then double clicks the node.
// The location must be stable for 1 iteration of polling (default 100ms).
func (n *Node) StableDoubleClick(ctx context.Context, pollOpts *testing.PollOptions) error {
	return n.stableMouseClick(ctx, doubleClick, pollOpts)
}

// clickType describes how user clicks mouse.
type clickType int

const (
	leftClick clickType = iota
	rightClick
	doubleClick
)

const clickNode = `
	async function(button){
		var loc = this.location;
		var centerpoint = {"x": loc.left + loc.width/2, "y": loc.top + loc.height/2};
		await tast.promisify(chrome.autotestPrivate.mouseMove)(centerpoint, 0);
		await tast.promisify(chrome.autotestPrivate.mouseClick)(button);
	}`

func (n *Node) mouseClick(ctx context.Context, ct clickType) error {
	switch ct {
	case leftClick:
		return n.object.Call(ctx, nil, clickNode, mouse.LeftButton)
	case rightClick:
		return n.object.Call(ctx, nil, clickNode, mouse.RightButton)
	case doubleClick:
		if err := n.Update(ctx); err != nil {
			return errors.Wrap(err, "failed to update the node's location")
		}
		if n.Location.Empty() {
			return errors.New("This node doesn't have a location on the screen and can't be clicked")
		}
		return mouse.DoubleClick(ctx, n.tconn, n.Location.CenterPoint(), 100*time.Millisecond)
	default:
		return errors.New("invalid click type")
	}
}

func (n *Node) updateLocation(ctx context.Context) error {
	return n.object.Call(ctx, &n.Location, `function(){return this.location}`)
}

// WaitLocationStable waits for the nodes location to be the same for a single iteration of polling.
// Polling options work the same way as in testing.Poll().
func (n *Node) WaitLocationStable(ctx context.Context, pollOpts *testing.PollOptions) error {
	location := coords.Rect{}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := n.updateLocation(ctx); err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to update the node's location"))
		}
		if !cmp.Equal(n.Location, location) {
			location = n.Location
			return errors.New("node location still changing")
		}
		return nil
	}, pollOpts); err != nil {
		return errors.Wrap(err, "node location unstable")
	}
	return nil
}

// stableMouseClick waits for a nodes location to be stable before attempting to click it.
// The location must be stable for 1 iteration of polling (default 100ms).
func (n *Node) stableMouseClick(ctx context.Context, ct clickType, pollOpts *testing.PollOptions) error {
	if err := n.WaitLocationStable(ctx, pollOpts); err != nil {
		return err
	}
	return n.mouseClick(ctx, ct)
}

// LeftClickUntil repeatedly left clicks the node until the condition returns true with no error.
// This is useful for situations where there is no indication of whether the node is ready to receive clicks.
// The interval between clicks and the timeout can be specified using testing.PollOptions.
// todo(crbug/1099507): remove this function when we have a better indicator of clickability in general
func (n *Node) LeftClickUntil(ctx context.Context, condition func(context.Context) (bool, error), opts *testing.PollOptions) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		if result, err := condition(ctx); err != nil {
			return testing.PollBreak(err)
		} else if !result {
			if err := n.LeftClick(ctx); err != nil {
				return errors.Wrap(err, "failed to click node")
			}
			return errors.New("click may not have been received yet")
		}
		return nil
	}, opts)
}

// StableFind repeatedly finds the first matching node until the node's location if stable.
// It will relocate the node if unexpected error happens, such as an element being refreshed.
// Stable*Click and WaitLocationStable are alternative solutions if the node is not completely deleted and recreated.
// Side effect: using this function will increase the test duration.
// Error example: "Failed to update the node's location: unexpected end of JSON input".
// Error example: "Cannot read property 'left' of undefined".
func StableFind(ctx context.Context, tconn *chrome.TestConn, params FindParams, opts *testing.PollOptions) (node *Node, err error) {
	location := coords.Rect{}

	return node, testing.Poll(ctx, func(ctx context.Context) error {
		node, err = Find(ctx, tconn, params)
		if err != nil {
			return errors.Wrap(err, "failed to find node")
		}
		if !cmp.Equal(node.Location, location) {
			location = node.Location
			node.Release(ctx)
			return errors.New("node location still changing")
		}
		return nil
	}, opts)
}

// StableFindAndClick waits for the first matching stable node and then left clicks it.
func StableFindAndClick(ctx context.Context, tconn *chrome.TestConn, params FindParams, opts *testing.PollOptions) error {
	node, err := StableFind(ctx, tconn, params, opts)
	if err != nil {
		return errors.Wrapf(err, "failed to find stable node with %v", params)
	}
	defer node.Release(ctx)
	return node.LeftClick(ctx)
}

// StableFindAndRightClick waits for the first matching stable node and then right clicks it.
func StableFindAndRightClick(ctx context.Context, tconn *chrome.TestConn, params FindParams, opts *testing.PollOptions) error {
	node, err := StableFind(ctx, tconn, params, opts)
	if err != nil {
		return errors.Wrapf(err, "failed to find stable node with %v", params)
	}
	defer node.Release(ctx)
	return node.RightClick(ctx)
}

// FocusAndWait calls the focus() Javascript method of the AutomationNode.
// This can be used to scroll to nodes which aren't currently visible, enabling them to be clicked.
// The focus event is not instant, so an EventWatcher (watcher.go) is used to check its status.
// The EventWatcher waits the duration of timeout for the event to occur.
func (n *Node) FocusAndWait(ctx context.Context, timeout time.Duration) error {
	ew, err := NewWatcher(ctx, n, EventTypeFocus)
	if err != nil {
		return errors.Wrap(err, "failed to create focus event watcher")
	}
	defer ew.Release(ctx)

	if err := n.object.Call(ctx, nil, "function(){this.focus()}"); err != nil {
		return errors.Wrap(err, "failed to call focus() on the specified node")
	}

	if _, err := ew.WaitForEvent(ctx, timeout); err != nil {
		return errors.Wrap(err, "failed to wait for the focus event on the specified node")
	}

	return nil
}

// MakeVisible calls makeVisible() Javascript method of the AutomationNode to make
// target node visible.
func (n *Node) MakeVisible(ctx context.Context) error {
	if err := n.object.Call(ctx, nil, "function(){return this.makeVisible()}"); err != nil {
		return errors.Wrap(err, "failed to call makeVisible() on the specified node")
	}
	return nil
}

// Descendant finds the first descendant of this node matching the params and returns it.
// If the JavaScript fails to execute, an error is returned.
func (n *Node) Descendant(ctx context.Context, params FindParams) (*Node, error) {
	paramsBytes, err := params.rawBytes()
	if err != nil {
		return nil, err
	}
	obj := &chrome.JSObject{}
	if err := n.object.Call(ctx, obj, fmt.Sprintf("function(){return this.find(%s)}", paramsBytes)); err != nil {
		return nil, err
	}
	return NewNode(ctx, n.tconn, obj)
}

// Descendants finds all descendant of this node matching the params and returns them.
// If the JavaScript fails to execute, an error is returned.
func (n *Node) Descendants(ctx context.Context, params FindParams) (NodeSlice, error) {
	paramsBytes, err := params.rawBytes()
	if err != nil {
		return nil, err
	}
	nodeList := &chrome.JSObject{}
	if err := n.object.Call(ctx, nodeList, fmt.Sprintf("function(){return this.findAll(%s)}", paramsBytes)); err != nil {
		return nil, err
	}
	defer nodeList.Release(ctx)

	var len int
	if err := nodeList.Call(ctx, &len, "function(){return this.length}"); err != nil {
		return nil, err
	}

	var nodes NodeSlice
	for i := 0; i < len; i++ {
		node, err := func() (*Node, error) {
			obj := &chrome.JSObject{}
			if err := nodeList.Call(ctx, obj, "function(i){return this[i]}", i); err != nil {
				nodes.Release(ctx)
				return nil, err
			}
			return NewNode(ctx, n.tconn, obj)
		}()
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}

// DescendantWithTimeout finds a descendant of this node using params and returns it.
// If the timeout is hit or the JavaScript fails to execute, an error is returned.
func (n *Node) DescendantWithTimeout(ctx context.Context, params FindParams, timeout time.Duration) (*Node, error) {
	if err := n.WaitUntilDescendantExists(ctx, params, timeout); err != nil {
		return nil, err
	}
	return n.Descendant(ctx, params)
}

// DescendantExists checks if a descendant of this node can be found.
// If the JavaScript fails to execute, an error is returned.
func (n *Node) DescendantExists(ctx context.Context, params FindParams) (bool, error) {
	paramsBytes, err := params.rawBytes()
	if err != nil {
		return false, err
	}
	var exists bool
	if err = n.object.Call(ctx, &exists, fmt.Sprintf("function(){return !!(this.find(%s))}", paramsBytes)); err != nil {
		return false, err
	}
	return exists, nil
}

// ErrNodeDoesNotExist is returned when the node is not found.
var ErrNodeDoesNotExist = errors.New("node does not exist")

// WaitUntilDescendantExists checks if a descendant node exists repeatedly until the timeout.
// If the timeout is hit or the JavaScript fails to execute, an error is returned.
func (n *Node) WaitUntilDescendantExists(ctx context.Context, params FindParams, timeout time.Duration) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		exists, err := n.DescendantExists(ctx, params)
		if err != nil {
			return testing.PollBreak(err)
		}
		if !exists {
			return ErrNodeDoesNotExist
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout})
}

// ErrNodeExists is returned when the node is found, but should not exist.
var ErrNodeExists = errors.New("node still exists")

// WaitUntilDescendantGone checks if a descendant node doesn't exist repeatedly until the timeout.
// If the timeout is hit or the JavaScript fails to execute, an error is returned.
func (n *Node) WaitUntilDescendantGone(ctx context.Context, params FindParams, timeout time.Duration) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		exists, err := n.DescendantExists(ctx, params)
		if err != nil {
			return testing.PollBreak(err)
		}
		if exists {
			return ErrNodeExists
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout})
}

// Matches returns whether this node matches the given params.
func (n *Node) Matches(ctx context.Context, params FindParams) (bool, error) {
	paramsBytes, err := params.rawBytes()
	if err != nil {
		return false, err
	}
	var match bool
	if err := n.object.Call(ctx, &match, fmt.Sprintf("function(){return this.matches(%s)}", paramsBytes)); err != nil {
		return false, err
	}
	return match, nil
}

// Attribute gets the specified attribute of this node.
// This method is for odd/uncommon attributes. For common attributes, add them to the Node struct.
// If the JavaScript fails to execute, an error is returned.
func (n *Node) Attribute(ctx context.Context, attributeName string) (interface{}, error) {
	var out interface{}

	if err := n.object.Call(ctx, &out, "function(attr){return this[attr]}", attributeName); err != nil {
		return nil, err
	}
	return out, nil
}

// Children returns the children of the node.
// If the JavaScript fails to execute, an error is returned.
func (n *Node) Children(ctx context.Context) (NodeSlice, error) {
	childrenList := &chrome.JSObject{}
	if err := n.object.Call(ctx, childrenList, "function(){return this.children}"); err != nil {
		return nil, errors.Wrap(err, "failed to call children() on the specified node")
	}
	defer childrenList.Release(ctx)

	var numChildren int
	if err := childrenList.Call(ctx, &numChildren, "function(){return this.length}"); err != nil {
		return nil, err
	}

	var children NodeSlice
	for i := 0; i < numChildren; i++ {
		node, err := func() (*Node, error) {
			currChild := &chrome.JSObject{}
			if err := childrenList.Call(ctx, currChild, "function(i){return this[i]}", i); err != nil {
				return nil, err
			}
			return NewNode(ctx, n.tconn, currChild)
		}()
		if err != nil {
			children.Release(ctx)
			return nil, err
		}
		children = append(children, node)
	}
	return children, nil
}

// ToString returns string representation of node.
// If the JavaScript fails to execute, an error is returned.
func (n *Node) ToString(ctx context.Context) (string, error) {
	var nodeString string
	if err := n.object.Call(ctx, &nodeString, "function() {return this.toString()}"); err != nil {
		return "", err
	}
	return nodeString, nil
}

// Root returns the chrome.automation root as a Node.
// If the JavaScript fails to execute, an error is returned.
func Root(ctx context.Context, tconn *chrome.TestConn) (*Node, error) {
	obj := &chrome.JSObject{}
	if err := tconn.EvalPromise(ctx, "tast.promisify(chrome.automation.getDesktop)()", obj); err != nil {
		return nil, err
	}
	return NewNode(ctx, tconn, obj)
}

// Select sets the document selection to include everything between the two nodes at the offsets.
// If the JavaScript fails to execute, an error is returned.
func Select(ctx context.Context, startNode *Node, startOffset int, endNode *Node, endOffset int) error {
	return startNode.tconn.Call(ctx, nil, `function(anchorObject, anchorOffset, focusObject, focusOffset){
		chrome.automation.setDocumentSelection({anchorObject, anchorOffset, focusObject, focusOffset})
	}`, startNode.object, startOffset, endNode.object, endOffset)
}

// Find finds the first descendant of the root node matching the params and returns it.
// If the JavaScript fails to execute, an error is returned.
func Find(ctx context.Context, tconn *chrome.TestConn, params FindParams) (*Node, error) {
	root, err := Root(ctx, tconn)
	if err != nil {
		return nil, err
	}
	defer root.Release(ctx)
	return root.Descendant(ctx, params)
}

// FindAll finds all descendants of the root node matching the params and returns them.
// If the JavaScript fails to execute, an error is returned.
func FindAll(ctx context.Context, tconn *chrome.TestConn, params FindParams) (NodeSlice, error) {
	root, err := Root(ctx, tconn)
	if err != nil {
		return nil, err
	}
	defer root.Release(ctx)
	return root.Descendants(ctx, params)
}

// FindSingleton finds the only descendants of the root node matching the params and returns it.
// If the JavaScript fails to execute, an error is returned.
// This differs from Find in that if there are multiple matching nodes, it returns an error.
func FindSingleton(ctx context.Context, tconn *chrome.TestConn, params FindParams) (*Node, error) {
	nodes, err := FindAll(ctx, tconn, params)
	if err != nil {
		return nil, err
	} else if len(nodes) == 0 {
		return nil, errors.Errorf("failed to find ui node matching parameters %+v. If you want a list of ui nodes, try ui.RootDebugInfo(context.Context, chrome.TestConn)", params)
	} else if len(nodes) > 1 {
		for _, node := range nodes {
			defer node.Release(ctx)
		}
		return nil, errors.Errorf("found multiple ui nodes matching parameters %+v: %+v", params, nodes)
	}
	return nodes[0], nil
}

// FindWithTimeout finds a descendant of the root node using params and returns it.
// If the JavaScript fails to execute, an error is returned.
func FindWithTimeout(ctx context.Context, tconn *chrome.TestConn, params FindParams, timeout time.Duration) (*Node, error) {
	root, err := Root(ctx, tconn)
	if err != nil {
		return nil, err
	}
	defer root.Release(ctx)
	return root.DescendantWithTimeout(ctx, params, timeout)
}

// Exists checks if a descendant of the root node can be found.
// If the JavaScript fails to execute, an error is returned.
func Exists(ctx context.Context, tconn *chrome.TestConn, params FindParams) (bool, error) {
	root, err := Root(ctx, tconn)
	if err != nil {
		return false, err
	}
	defer root.Release(ctx)
	return root.DescendantExists(ctx, params)
}

// WaitUntilExists checks if a node exists repeatedly until the timeout.
// If the JavaScript fails to execute, an error is returned.
func WaitUntilExists(ctx context.Context, tconn *chrome.TestConn, params FindParams, timeout time.Duration) error {
	root, err := Root(ctx, tconn)
	if err != nil {
		return err
	}
	defer root.Release(ctx)
	return root.WaitUntilDescendantExists(ctx, params, timeout)
}

// WaitUntilGone checks if a node doesn't exist repeatedly until the timeout.
// If the JavaScript fails to execute, an error is returned.
func WaitUntilGone(ctx context.Context, tconn *chrome.TestConn, params FindParams, timeout time.Duration) error {
	root, err := Root(ctx, tconn)
	if err != nil {
		return err
	}
	defer root.Release(ctx)
	return root.WaitUntilDescendantGone(ctx, params, timeout)
}

// WaitUntilExistsStatus repeatedly checks the existence of a node
// until the desired status is found or the timeout is reached.
// If the JavaScript fails to execute, an error is returned.
func WaitUntilExistsStatus(ctx context.Context, tconn *chrome.TestConn, params FindParams, exists bool, timeout time.Duration) error {
	if exists {
		return WaitUntilExists(ctx, tconn, params, timeout)
	}

	return WaitUntilGone(ctx, tconn, params, timeout)
}

// RootDebugInfo returns the chrome.automation root as a string.
// If the JavaScript fails to execute, an error is returned.
func RootDebugInfo(ctx context.Context, tconn *chrome.TestConn) (string, error) {
	var out string
	err := tconn.EvalPromise(ctx, "tast.promisify(chrome.automation.getDesktop)().then(root => root+'');", &out)
	return out, err
}

// LogRootDebugInfo logs the chrome.automation root debug info to a file.
func LogRootDebugInfo(ctx context.Context, tconn *chrome.TestConn, filename string) error {
	debugInfo, err := RootDebugInfo(ctx, tconn)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filename, []byte(debugInfo), 0644)
}

// ErrNoRadioButtons is returned when there are no radio buttons under the radio group.
var ErrNoRadioButtons = errors.New("no radio buttons in this radio group")

// ErrNoActiveRadioButton is returned when no active radio group is selected.
var ErrNoActiveRadioButton = errors.New("no active radio button is selected")

// FindSelectedRadioButton finds the selected radio button under a radio group node.
// If no active radio button is selected, an error is returned.
func (n *Node) FindSelectedRadioButton(ctx context.Context) (*Node, error) {
	if n.Role != RoleTypeRadioGroup {
		return nil, errors.New("the current node is not a radio group")
	}

	if rbExists, err := n.DescendantExists(ctx, FindParams{Role: RoleTypeRadioButton}); err != nil {
		return nil, errors.Wrap(err, "failed to run javascript to check if radio buttons exist")
	} else if rbExists == false {
		return nil, ErrNoRadioButtons
	}

	if selectedExists, err := n.DescendantExists(ctx, FindParams{
		Role:       RoleTypeRadioButton,
		Attributes: map[string]interface{}{"checked": CheckedStateTrue},
	}); err != nil {
		return nil, errors.Wrap(err, "failed to run javascript to check if a selected radio button exist")
	} else if selectedExists == false {
		return nil, ErrNoActiveRadioButton
	}

	return n.Descendant(ctx, FindParams{
		Role:       RoleTypeRadioButton,
		Attributes: map[string]interface{}{"checked": CheckedStateTrue},
	})
}
