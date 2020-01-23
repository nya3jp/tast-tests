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
	"regexp"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
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
		case string, RoleType:
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

// Rect represents the bounds of a Node
// As defined in chromium/src/extensions/common/api/automation.idl
type Rect struct {
	Left   int `json:"left"`
	Top    int `json:"top"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// Node is a reference to chrome.automation API AutomationNode.
// Node intentionally leaves out many properties. If they become needed, add them to the Node struct and to the newNode function.
// As defined in chromium/src/extensions/common/api/automation.idl
type Node struct {
	object    *chrome.JSObject
	conn      *chrome.Conn
	Name      string             `json:"name,omitempty"`
	ClassName string             `json:"classname,omitempty"`
	Role      RoleType           `json:"role,omitempty"`
	State     map[StateType]bool `json:"state,omitempty"`
	Location  *Rect              `json:"location,omitempty"`
}

// newNode creates a new node struct and initializes its fields.
// newNode takes ownership of obj and will release it if the node fails to initialize.
func newNode(ctx context.Context, conn *chrome.Conn, obj *chrome.JSObject) (*Node, error) {
	node := &Node{
		object: obj,
		conn:   conn,
	}
	if err := node.object.Call(ctx, node, `function(){
		return {
			name: this.name,
			classname: this.classname,
			role: this.role,
			state: this.state,
			location: this.location}
		}`); err != nil {
		node.Release(ctx)
		return nil, errors.Wrap(err, "failed to initialize node")
	}
	return node, nil
}

// Release frees the reference to Javascript for this node.
func (n *Node) Release(ctx context.Context) {
	n.object.Release(ctx)
}

// LeftClick executes the default action of the node.
// If the JavaScript fails to execute, an error is returned.
func (n *Node) LeftClick(ctx context.Context) error {
	if n.Location == nil {
		return errors.New("this node doesn't have a location on the screen and can't be clicked")
	}
	return ash.MouseClick(ctx, n.conn, ash.Location{X: n.Location.Left + n.Location.Width/2, Y: n.Location.Top + n.Location.Height/2}, ash.LeftButton)
}

// RightClick shows the context menu of the node.
// If the JavaScript fails to execute, an error is returned.
func (n *Node) RightClick(ctx context.Context) error {
	if n.Location == nil {
		return errors.New("this node doesn't have a location on the screen and can't be clicked")
	}
	return ash.MouseClick(ctx, n.conn, ash.Location{X: n.Location.Left + n.Location.Width/2, Y: n.Location.Top + n.Location.Height/2}, ash.RightButton)
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
	return newNode(ctx, n.conn, obj)
}

// Descendants finds all descendant of this node matching the params and returns them.
// If the JavaScript fails to execute, an error is returned.
func (n *Node) Descendants(ctx context.Context, params FindParams) ([]*Node, error) {
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

	var nodes []*Node
	var initErr error
	for i := 0; i < len; i++ {
		obj := &chrome.JSObject{}
		if err := nodeList.Call(ctx, obj, "function(i){return this[i]}", i); err != nil {
			initErr = err
			break
		}
		node, err := newNode(ctx, n.conn, obj)
		if err != nil {
			initErr = err
			break
		}
		nodes = append(nodes, node)
	}
	// On error, release all nodes.
	if initErr != nil {
		for _, node := range nodes {
			node.Release(ctx)
		}
		return nil, initErr
	}
	return nodes, nil
}

// DescendantWithTimeout finds a descendant of this node using params and returns it.
// If the timeout is hit or the JavaScript fails to execute, an error is returned.
func (n *Node) DescendantWithTimeout(ctx context.Context, params FindParams, timeout time.Duration) (*Node, error) {
	if err := n.WaitForDescendant(ctx, params, true, timeout); err != nil {
		return nil, err
	}
	return n.Descendant(ctx, params)
}

// DescendantExists checks if a node can be found.
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

// WaitForDescendant checks for a node repeatly until the timeout.
// If "exists" is true, it will wait for the descendent to exist.
// Otherwise, it will wait for the descendent to no longer exist.
// If the timeout is hit or the JavaScript fails to execute, an error is returned.
func (n *Node) WaitForDescendant(ctx context.Context, params FindParams, exists bool, timeout time.Duration) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		actual, err := n.DescendantExists(ctx, params)
		if err != nil {
			return testing.PollBreak(err)
		}
		if exists && !actual {
			return errors.New("node does not exist")
		} else if !exists && actual {
			return errors.New("node still exist")
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout}); err != nil {
		return errors.Wrap(err, "failed to wait for the node")
	}
	return nil
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

// Root returns the chrome.automation root as a Node.
// If the JavaScript fails to execute, an error is returned.
func Root(ctx context.Context, conn *chrome.Conn) (*Node, error) {
	obj := &chrome.JSObject{}
	if err := conn.EvalPromise(ctx, "tast.promisify(chrome.automation.getDesktop)()", obj); err != nil {
		return nil, err
	}
	return newNode(ctx, conn, obj)
}

// RootDebugInfo returns the chrome.automation root as a string.
// If the JavaScript fails to execute, an error is returned.
func RootDebugInfo(ctx context.Context, conn *chrome.Conn) (string, error) {
	var out string
	err := conn.EvalPromise(ctx, "tast.promisify(chrome.automation.getDesktop)().then(root => root+'');", &out)
	return out, err
}
