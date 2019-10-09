// Copyright 2019 The Chromium OS Authors. All rights reserved.
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
	"strings"
	"time"

	"github.com/mafredri/cdp/protocol/runtime"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/testing"
)

// FindParams is a mapping of chrome.automation.FindParams to Golang.
// Name and ClassName allow quick access because they are common attributes.
// As defined in https://chromium.googlesource.com/chromium/src/+/refs/heads/master/extensions/common/api/automation.idl#436
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

	// json.Marshal can't be used because it doesn't support JavaScript regular expression notation.
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
			return nil, errors.Errorf("FindParams does not support type: %T", v)
		}
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// rawBytes converts FindParams into a JSON like object that can contain JS Regexp Notation.
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
// As defined in https://chromium.googlesource.com/chromium/src/+/refs/heads/master/extensions/common/api/automation.idl#428
type Rect struct {
	Left   int `json:"left"`
	Top    int `json:"top"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// Node is a reference to chrome.automation API AutomationNode.
// Node intentionally leaves out many properties. If they become needed, add them here.
// As defined in https://chromium.googlesource.com/chromium/src/+/refs/heads/master/extensions/common/api/automation.idl#548
type Node struct {
	object    runtime.RemoteObject
	tconn     *chrome.Conn
	Name      string             `json:"name"`
	ClassName string             `json:"classname"`
	Role      *RoleType          `json:"role"`
	State     map[StateType]bool `json:"state"`
	Location  *Rect              `json:"location"`
}

// Init pulls the properties of a Node
func (n *Node) Init(ctx context.Context) error {
	_, err := n.tconn.CallFunctionOn(ctx, *n.object.ObjectID, `function(){
		return {
			name: this.name,
			role: this.role,
			location: this.location,
			state: this.state,
			classname: this.classname}
		}`, nil, &n)
	return err
}

// Release frees the reference to Javascript for this node.
func (n *Node) Release(ctx context.Context) {
	n.tconn.ReleaseObject(ctx, n.object)
}

// LeftClick executes the default action of the node.
// If the JavaScript fails to execute, an error is returned.
func (n *Node) LeftClick(ctx context.Context) error {
	if n.Location == nil {
		return errors.New("this node doesn't have a location on the screen and can't be clicked")
	}
	return ash.MouseClick(ctx, n.tconn, ash.Location{X: n.Location.Left + n.Location.Width/2, Y: n.Location.Top + n.Location.Height/2}, ash.LeftButton)
}

// RightClick shows the context menu of the node.
// If the JavaScript fails to execute, an error is returned.
func (n *Node) RightClick(ctx context.Context) error {
	if n.Location == nil {
		return errors.New("this node doesn't have a location on the screen and can't be clicked")
	}
	return ash.MouseClick(ctx, n.tconn, ash.Location{X: n.Location.Left + n.Location.Width/2, Y: n.Location.Top + n.Location.Height/2}, ash.RightButton)
}

// Descendant finds a descendant of this node using params and returns it.
// If the JavaScript fails to execute, an error is returned.
func (n *Node) Descendant(ctx context.Context, params FindParams) (*Node, error) {
	paramsBytes, err := params.rawBytes()
	if err != nil {
		return nil, err
	}

	object, err := n.tconn.CallFunctionOn(ctx, *n.object.ObjectID, fmt.Sprintf("function(){return this.find(%s)}", paramsBytes), nil, nil)
	if err != nil {
		return nil, err
	}
	if object == nil || object.ObjectID == nil {
		return nil, errors.New("node descendant not found")
	}
	node := Node{
		object: *object,
		tconn:  n.tconn,
	}
	return &node, node.Init(ctx)
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
	_, err = n.tconn.CallFunctionOn(ctx, *n.object.ObjectID, fmt.Sprintf("function(){return !!(this.find(%s))}", paramsBytes), nil, &exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

// WaitForDescendant checks for a node repeatly until the timeout.
// The parameter exists decides if it polls for the node to exist or until it no longer exists
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

// Attributes gets the specified attributes of this node.
// This method is for odd/uncommon attributes. For common attributes, add them to the Node struct.
// If the JavaScript fails to execute, an error is returned.
func (n *Node) Attributes(ctx context.Context, attributes []string) (map[string]interface{}, error) {
	attr := fmt.Sprintf("{%s}", strings.Join(attributes, ","))
	var out map[string]interface{}
	// This uses JavasScript destructuring assignment to grab only the wanted attributes.
	object, err := n.tconn.CallFunctionOn(ctx, *n.object.ObjectID, fmt.Sprintf("function(){return ((%[1]s) => (%[1]s))(this)}", attr), nil, &out)
	if err != nil {
		return nil, err
	}
	n.tconn.ReleaseObject(ctx, *object)
	return out, nil
}

// DescendantAttributes gets the specified attributes of all matching nodes.
// If the JavaScript fails to execute, an error is returned.
func (n *Node) DescendantAttributes(ctx context.Context, params FindParams, attributes []string) ([]map[string]interface{}, error) {
	attr := fmt.Sprintf("{%s}", strings.Join(attributes, ","))
	paramsBytes, err := params.rawBytes()
	if err != nil {
		return nil, err
	}
	var out []map[string]interface{}
	// This uses JavasScript destructuring assignment to grab only the wanted attributes.
	object, err := n.tconn.CallFunctionOn(ctx, *n.object.ObjectID, fmt.Sprintf("function(){return this.findAll(%[1]s).map((%[2]s) => (%[2]s))}", paramsBytes, attr), nil, &out)
	if err != nil {
		return nil, err
	}
	n.tconn.ReleaseObject(ctx, *object)
	return out, nil
}

// Root returns the chrome.automation root as a Node.
// If the JavaScript fails to execute, an error is returned.
func Root(ctx context.Context, tconn *chrome.Conn) (*Node, error) {
	object, err := tconn.RemoteObject(ctx, "tast.promisify(chrome.automation.getDesktop)()")
	if err != nil {
		return nil, err
	}
	if object.ObjectID == nil {
		return nil, errors.New("root node not found, objectID was nil")
	}
	root := Node{
		object: *object,
		tconn:  tconn,
	}
	return &root, root.Init(ctx)
}

// RootDebugInfo returns the chrome.automation root as a string.
// If the JavaScript fails to execute, an error is returned.
func RootDebugInfo(ctx context.Context, tconn *chrome.Conn) (string, error) {
	var out string
	err := tconn.EvalPromise(ctx, "tast.promisify(chrome.automation.getDesktop)().then(root => root+'');", &out)
	return out, err
}
