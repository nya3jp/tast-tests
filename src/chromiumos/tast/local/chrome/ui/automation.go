// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

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
type FindParams struct {
	Role       string
	Name       string
	ClassName  string
	Attributes map[string]interface{}
	State      map[string]bool
	Parent     *FindParams
}

func (params *FindParams) toJSON() (string, error) {
	if params.Attributes == nil {
		params.Attributes = make(map[string]interface{})
	}
	// Ensure parameters aren't passed twice.
	if params.Role != "" {
		if _, exists := params.Attributes["role"]; exists {
			return "", errors.New("cannot set both FindParams.Role and FindParams.Attributes['role']")
		}
		params.Attributes["role"] = params.Role
	}
	if params.Name != "" {
		if _, exists := params.Attributes["name"]; exists {
			return "", errors.New("cannot set both FindParams.Name and FindParams.Attributes['name']")
		}
		params.Attributes["name"] = params.Name
	}
	if params.ClassName != "" {
		if _, exists := params.Attributes["className"]; exists {
			return "", errors.New("cannot set both FindParams.ClassName and FindParams.Attributes['className']")
		}
		params.Attributes["className"] = params.ClassName
	}
	// params.Attributes can't use json.Marshal because regexp.Regexp is not supported.
	attr := "{"
	for k, v := range params.Attributes {
		switch v := v.(type) {
		case string:
			attr += fmt.Sprintf("%q:%q,", k, v)
		case int:
			attr += fmt.Sprintf("%q:%d,", k, v)
		case float32:
			attr += fmt.Sprintf("%q:%f,", k, v)
		case float64:
			attr += fmt.Sprintf("%q:%f,", k, v)
		case bool:
			attr += fmt.Sprintf("%q:%t,", k, v)
		case regexp.Regexp:
			attr += fmt.Sprintf(`%q:/%s/,`, k, v.String())
		case *regexp.Regexp:
			attr += fmt.Sprintf(`%q:/%s/,`, k, v.String())
		default:
			return "", errors.Errorf("FindParams does not support type: %T", v)
		}
	}
	attr += "}"
	state, err := json.Marshal(params.State)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(`{"attributes":%s,"state":%s}`, attr, state), nil
}

// Node is a reference to chrome.automation API Automation Node.
type Node struct {
	object runtime.RemoteObject
	tconn  *chrome.Conn
}

// Release frees the reference to Javascript for this node.
func (n *Node) Release(ctx context.Context) {
	n.tconn.ReleaseObject(ctx, n.object)
}

// LeftClick executes the default action of the node with the specific FindParams.
// If the JavaScript fails to execute, an error is returned.
func (n *Node) LeftClick(ctx context.Context) error {
	_, err := n.tconn.CallFunctionOn(ctx, *n.object.ObjectID, "function(){this.doDefault()}", nil, false, nil)
	return err
}

// RightClick shows the context menu of the node with the specific FindParams.
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
	if err := n.WaitForDescendantToAppear(ctx, params, timeout); err != nil {
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

// WaitForDescendantToAppear checks for a node repeatly until either the timeout or it exists.
// If the JavaScript fails to execute, an error is returned.
func (n *Node) WaitForDescendantToAppear(ctx context.Context, params FindParams, timeout time.Duration) error {
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

// WaitForDescendantToDisappear checks for a node repeatly until either the timeout or it doesn't exists.
// If the JavaScript fails to execute, an error is returned.
func (n *Node) WaitForDescendantToDisappear(ctx context.Context, params FindParams, timeout time.Duration) error {
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
	attr := "{"
	attr += strings.Join(attributes, ",")
	attr += "}"
	var out map[string]interface{}
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
	attr := "{"
	attr += strings.Join(attributes, ",")
	attr += "}"
	paramsJSON, err := params.toJSON()
	if err != nil {
		return nil, err
	}
	var out []map[string]interface{}
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
