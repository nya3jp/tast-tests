// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package a11y

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/checked"
	"chromiumos/tast/local/chrome/uiauto/restriction"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/state"
	"chromiumos/tast/testing"
)

// FindParams is a mapping of chrome.automation.FindParams to Golang.
// As defined in chromium/src/extensions/common/api/automation.idl
type FindParams struct {
	Name       string
	Role       role.Role
	Attributes map[string]interface{}
	State      map[state.State]bool
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

	// Return null if empty dictionary
	if len(attributes) == 0 {
		return []byte("null"), nil
	}

	// json.Marshal can't be used because this is JavaScript code with regular expressions, not JSON.
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
		case string, role.Role, checked.Checked, restriction.Restriction:
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
	object *chrome.JSObject
	tconn  *chrome.TestConn
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
	return &Node{
		object: obj,
		tconn:  tconn,
	}, nil
}

// Release frees the reference to Javascript for this node.
func (n *Node) Release(ctx context.Context) {
	n.object.Release(ctx)
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
	if err := tconn.Call(ctx, obj, "tast.promisify(chrome.automation.getDesktop)"); err != nil {
		return nil, err
	}
	return NewNode(ctx, tconn, obj)
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

// StandardActions returns the standard actions of the node.
// If the JavaScript fails to execute, an error is returned.
func (n *Node) StandardActions(ctx context.Context) ([]string, error) {
	actionList := &chrome.JSObject{}
	if err := n.object.Call(ctx, actionList, "function(){return this.standardActions}"); err != nil {
		return nil, errors.Wrap(err, "failed to call standardActions() on the specified node")
	}
	defer actionList.Release(ctx)

	var numActions int
	if err := actionList.Call(ctx, &numActions, "function(){return this.length}"); err != nil {
		return nil, errors.Wrap(err, "failed to call length() on standard actions")
	}

	var actions []string
	for i := 0; i < numActions; i++ {
		var action string
		if err := actionList.Call(ctx, &action, "function(i){return this[i]}", i); err != nil {
			return nil, errors.Wrap(err, "failed to call this[i] on standard actions")
		}
		actions = append(actions, action)
	}

	return actions, nil
}
