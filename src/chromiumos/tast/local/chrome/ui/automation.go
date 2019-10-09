// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ui enables interacting with the ChromeOS UI through the chrome.automation API.
// The chrome.automation API is documented here: https://developer.chrome.com/extensions/automation
package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/mafredri/cdp/protocol/runtime"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// FindParams is a mapping of chrome.automation.FindParams to Golang.
// Role, Name, and ClassName allow quick access because they are common attributes.
// As defined in https://chromium.googlesource.com/chromium/src/+/refs/heads/master/extensions/common/api/automation.idl#436
type FindParams struct {
	Role       RoleType
	Name       string
	ClassName  string
	Attributes map[string]interface{}
	State      map[StateType]bool
}

// getFullAttributes creates a map of Attributes including Role, Name, and ClassName.
// If any attribute is defined twice, an error is returned.
func (params *FindParams) getFullAttributes() (map[string]interface{}, error) {
	out := make(map[string]interface{})
	if params.Attributes != nil {
		for k, v := range params.Attributes {
			out[k] = v
		}
	}
	// Ensure parameters aren't passed twice.
	if params.Role != "" {
		if _, exists := out["role"]; exists {
			return nil, errors.New("cannot set both FindParams.Role and FindParams.Attributes['role']")
		}
		out["role"] = params.Role
	}
	if params.Name != "" {
		if _, exists := out["name"]; exists {
			return nil, errors.New("cannot set both FindParams.Name and FindParams.Attributes['name']")
		}
		out["name"] = params.Name
	}
	if params.ClassName != "" {
		if _, exists := out["className"]; exists {
			return nil, errors.New("cannot set both FindParams.ClassName and FindParams.Attributes['className']")
		}
		out["className"] = params.ClassName
	}
	return out, nil
}

func (params *FindParams) toJSON() (string, error) {
	attributes, err := params.getFullAttributes()
	if err != nil {
		return "", err
	}
	// json.Marshal can't be used because it doesn't support Javascript regular expression notations.
	var attrBuilder strings.Builder
	attrBuilder.WriteByte('{')
	for k, v := range attributes {
		switch v := v.(type) {
		case string, RoleType:
			fmt.Fprintf(&attrBuilder, "%q:%q,", k, v)
		case int, float32, float64, bool:
			fmt.Fprintf(&attrBuilder, "%q:%v,", k, v)
		case regexp.Regexp, *regexp.Regexp:
			fmt.Fprintf(&attrBuilder, `%q:/%v/,`, k, v)
		default:
			return "", errors.Errorf("FindParams does not support type: %T", v)
		}
	}
	attrBuilder.WriteByte('}')
	state, err := json.Marshal(params.State)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(`{"attributes":%s,"state":%s}`, attrBuilder.String(), state), nil
}

// Node is a reference to chrome.automation API AutomationNode.
// As defined in https://chromium.googlesource.com/chromium/src/+/refs/heads/master/extensions/common/api/automation.idl#548
type Node struct {
	object runtime.RemoteObject
	tconn  *chrome.Conn
}

// Release frees the reference to Javascript for this node.
func (n *Node) Release(ctx context.Context) {
	n.tconn.ReleaseObject(ctx, n.object)
}

// LeftClick executes the default action of the node.
// If the JavaScript fails to execute, an error is returned.
func (n *Node) LeftClick(ctx context.Context) error {
	_, err := n.tconn.CallFunctionOn(ctx, *n.object.ObjectID, "function(){this.doDefault()}", nil, false, nil)
	return err
}

// RightClick shows the context menu of the node.
// If the JavaScript fails to execute, an error is returned.
func (n *Node) RightClick(ctx context.Context) error {
	_, err := n.tconn.CallFunctionOn(ctx, *n.object.ObjectID, "function(){this.showContextMenu()}", nil, false, nil)
	return err
}

// GetDescendant finds a descendant of this node using params and returns it.
// If the JavaScript fails to execute, an error is returned.
func (n *Node) GetDescendant(ctx context.Context, params FindParams) (*Node, error) {
	paramsJSON, err := params.toJSON()
	if err != nil {
		return nil, err
	}
	object, err := n.tconn.CallFunctionOn(ctx, *n.object.ObjectID, fmt.Sprintf("function(){return this.find(%s)}", paramsJSON), nil, false, nil)
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
	return &node, nil
}

// GetDescendantWithTimeout finds a descendant of this node using params and returns it.
// If the timeout is hit or the JavaScript fails to execute, an error is returned.
func (n *Node) GetDescendantWithTimeout(ctx context.Context, params FindParams, timeout time.Duration) (*Node, error) {
	if err := n.WaitForDescendantAdded(ctx, params, timeout); err != nil {
		return nil, err
	}
	return n.GetDescendant(ctx, params)
}

// DescendantExists checks if a node can be found.
// If the JavaScript fails to execute, an error is returned.
func (n *Node) DescendantExists(ctx context.Context, params FindParams) (bool, error) {
	paramsJSON, err := params.toJSON()
	if err != nil {
		return false, err
	}
	var exists bool
	_, err = n.tconn.CallFunctionOn(ctx, *n.object.ObjectID, fmt.Sprintf("function(){return !!(this.find(%s))}", paramsJSON), nil, false, &exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

// WaitForDescendantAdded checks for a node repeatly until either the timeout or it exists.
// If the timeout is hit or the JavaScript fails to execute, an error is returned.
func (n *Node) WaitForDescendantAdded(ctx context.Context, params FindParams, timeout time.Duration) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		exists, err := n.DescendantExists(ctx, params)
		if err != nil {
			return err
		}
		if !exists {
			return errors.New("node does not exist")
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout}); err != nil {
		return errors.Wrap(err, "failed to wait for the node to appear")
	}
	return nil
}

// WaitForDescendantRemoved checks for a node repeatly until either the timeout or it doesn't exists.
// If the timeout is hit or the JavaScript fails to execute, an error is returned.
func (n *Node) WaitForDescendantRemoved(ctx context.Context, params FindParams, timeout time.Duration) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		exists, err := n.DescendantExists(ctx, params)
		if err != nil {
			return err
		}
		if exists {
			return errors.New("node still exist")
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout}); err != nil {
		return errors.Wrap(err, "failed to wait for the node to disappear")
	}
	return nil
}

// GetAttributes gets the specified attributes of this node.
// If the JavaScript fails to execute, an error is returned.
func (n *Node) GetAttributes(ctx context.Context, attributes []string) (map[string]interface{}, error) {
	attr := fmt.Sprintf("{%s}", strings.Join(attributes, ","))
	var out map[string]interface{}
	// This uses JavasScript destructuring assignment to grab only the wanted attributes.
	object, err := n.tconn.CallFunctionOn(ctx, *n.object.ObjectID, fmt.Sprintf("function(){return ((%[1]s) => (%[1]s))(this)}", attr), nil, false, &out)
	if err != nil {
		return nil, err
	}
	n.tconn.ReleaseObject(ctx, *object)
	return out, nil
}

// GetDescendantAttributes gets the specified attributes of all matching nodes.
// If the JavaScript fails to execute, an error is returned.
func (n *Node) GetDescendantAttributes(ctx context.Context, params FindParams, attributes []string) ([]map[string]interface{}, error) {
	attr := fmt.Sprintf("{%s}", strings.Join(attributes, ","))
	paramsJSON, err := params.toJSON()
	if err != nil {
		return nil, err
	}
	var out []map[string]interface{}
	// This uses JavasScript destructuring assignment to grab only the wanted attributes.
	object, err := n.tconn.CallFunctionOn(ctx, *n.object.ObjectID, fmt.Sprintf("function(){return this.findAll(%[1]s).map((%[2]s) => (%[2]s))}", paramsJSON, attr), nil, false, &out)
	if err != nil {
		return nil, err
	}
	n.tconn.ReleaseObject(ctx, *object)
	return out, nil
}

// Root returns the chrome.automation root as a Node.
// If the JavaScript fails to execute, an error is returned.
func Root(ctx context.Context, tconn *chrome.Conn) (*Node, error) {
	object, err := tconn.GetRemoteObject(ctx, "tast.promisify(chrome.automation.getDesktop)()", true)
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
	return &root, err
}

// RootDebugInfo returns the chrome.automation root as a string.
// If the JavaScript fails to execute, an error is returned.
func RootDebugInfo(ctx context.Context, tconn *chrome.Conn) (string, error) {
	var out string
	err := tconn.EvalPromise(ctx, "tast.promisify(chrome.automation.getDesktop)().then(root => root+'');", &out)
	return out, err
}
