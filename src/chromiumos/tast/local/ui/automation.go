// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// FindParams is a mapping of chrome.automation.FindParams to Golang.
type FindParams struct {
	Attributes map[string]interface{}
	State      map[string]bool
}

func (fp *FindParams) toJSON() (string, error) {
	// fp.Attributes can't use json.Marshal because regexp.Regexp is not supported.
	attr := "{"
	for k, v := range fp.Attributes {
		switch v := v.(type) {
		case string:
			attr += fmt.Sprintf("%q: %q,", k, v)
		case int:
			attr += fmt.Sprintf("%q: %d,", k, v)
		case float32:
			attr += fmt.Sprintf("%q: %f,", k, v)
		case float64:
			attr += fmt.Sprintf("%q: %f,", k, v)
		case bool:
			attr += fmt.Sprintf("%q: %t,", k, v)
		case regexp.Regexp:
			attr += fmt.Sprintf("%q: /%s/,", k, v.String())
		case *regexp.Regexp:
			attr += fmt.Sprintf("%q: /%s/,", k, v.String())
		default:
			return "", errors.Errorf("FindParams does not support type: %T", v)
		}
	}
	attr += "}"
	state, err := json.Marshal(fp.State)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("{attributes: %s, state: %s}", attr, state), nil
}

// LeftClick executes the default action of the node with the specific FindParams.
// If the JavaScript fails to execute, an error is returned.
func LeftClick(ctx context.Context, tconn *chrome.Conn, params FindParams) error {
	json, err := params.toJSON()
	if err != nil {
		return err
	}
	query := fmt.Sprintf("tast.promisify(chrome.automation.getDesktop)().then(root => root.find(%s).doDefault());", json)
	return tconn.EvalPromise(ctx, query, nil)
}

// RightClick shows the context menu of the node with the specific FindParams.
// If the JavaScript fails to execute, an error is returned.
func RightClick(ctx context.Context, tconn *chrome.Conn, params FindParams) error {
	json, err := params.toJSON()
	if err != nil {
		return err
	}
	query := fmt.Sprintf("tast.promisify(chrome.automation.getDesktop)().then(root => root.find(%s).showContextMenu());", json)
	return tconn.EvalPromise(ctx, query, nil)
}

// NodeExists checks if a node can be found.
// If the JavaScript fails to execute, an error is returned.
func NodeExists(ctx context.Context, tconn *chrome.Conn, params FindParams) (bool, error) {
	json, err := params.toJSON()
	if err != nil {
		return false, err
	}
	var exists bool
	query := fmt.Sprintf("tast.promisify(chrome.automation.getDesktop)().then(root => !!root.find(%s));", json)
	err = tconn.EvalPromise(ctx, query, &exists)
	return exists, err
}

// WaitForNodeToAppear checks for a node repeatly until either the timeout or it exists.
// If the JavaScript fails to execute, an error is returned.
func WaitForNodeToAppear(ctx context.Context, tconn *chrome.Conn, params FindParams, timeout time.Duration) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		exists, err := NodeExists(ctx, tconn, params)
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

// WaitForNodeToDisappear checks for a node repeatly until either the timeout or it doesn't exists.
// If the JavaScript fails to execute, an error is returned.
func WaitForNodeToDisappear(ctx context.Context, tconn *chrome.Conn, params FindParams, timeout time.Duration) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		exists, err := NodeExists(ctx, tconn, params)
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

// GetNode gets the specified attributes of a node.
// If the JavaScript fails to execute, an error is returned.
func GetNode(ctx context.Context, tconn *chrome.Conn, params FindParams, attributes []string) (map[string]interface{}, error) {
	json, err := params.toJSON()
	if err != nil {
		return nil, err
	}
	attr := "{"
	for _, a := range attributes {
		attr += a + ","
	}
	attr += "}"
	var out map[string]interface{}
	query := fmt.Sprintf("tast.promisify(chrome.automation.getDesktop)().then(root => root.find(%[1]s)).then((%[2]s) => (%[2]s));", json, attr)
	err = tconn.EvalPromise(ctx, query, &out)
	return out, err
}

// GetAllNodes gets the specified attributes of all matching nodes.
// If the JavaScript fails to execute, an error is returned.
func GetAllNodes(ctx context.Context, tconn *chrome.Conn, params FindParams, attributes []string) ([]map[string]interface{}, error) {
	json, err := params.toJSON()
	if err != nil {
		return nil, err
	}
	attr := "{"
	for _, a := range attributes {
		attr += a + ","
	}
	attr += "}"
	var out []map[string]interface{}
	query := fmt.Sprintf("tast.promisify(chrome.automation.getDesktop)().then(root => root.findAll(%[1]s).map((%[2]s) => (%[2]s)));", json, attr)
	err = tconn.EvalPromise(ctx, query, &out)
	return out, err
}

// RootDebugInfo returns the chrome.automation root as a string.
// If the JavaScript fails to execute, an error is returned.
func RootDebugInfo(ctx context.Context, tconn *chrome.Conn) (string, error) {
	var out string
	err := tconn.EvalPromise(ctx, "tast.promisify(chrome.automation.getDesktop)().then(root => root+'');", &out)
	return out, err
}
