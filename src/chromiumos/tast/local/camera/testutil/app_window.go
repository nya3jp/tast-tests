// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package testutil provides utilities to setup testing environment for camera
// tests.
package testutil

import (
	"context"

	"chromiumos/tast/local/chrome"
)

// PerfEntry stores the information of a perf event.
type PerfEntry struct {
	Event    string  `json:"event"`
	Duration float64 `json:"duration"`
	PerfInfo struct {
		Facing string `json:"facing"`
	} `json:"perfInfo"`
}

// ErrorLevel represents the severity level of an error.
type ErrorLevel string

const (
	// ErrorLevelWarning is used when it should not cause test failure.
	ErrorLevelWarning ErrorLevel = "WARNING"
	// ErrorLevelError is used when it should fail the test.
	ErrorLevelError = "ERROR"
)

// ErrorInfo stores the information of an error.
type ErrorInfo struct {
	ErrorType string     `json:"type"`
	Level     ErrorLevel `json:"level"`
	Stack     string     `json:"stack"`
	Time      int64      `json:"time"`
}

// AppWindow is used to comminicate with CCA foreground window.
type AppWindow struct {
	jsObj *chrome.JSObject
}

// WaitUntilWindowBound waits until CCA binds its window to the AppWindow instance.
func (a *AppWindow) WaitUntilWindowBound(ctx context.Context) (string, error) {
	var windowURL string
	if err := a.jsObj.Call(ctx, &windowURL, "function() { return this.waitUntilWindowBound(); }"); err != nil {
		return "", err
	}
	return windowURL, nil
}

// NotifyReady notifies CCA that the setup on test side is ready so it can continue the execution.
func (a *AppWindow) NotifyReady(ctx context.Context) error {
	return a.jsObj.Call(ctx, nil, "function() { return this.notifyReadyOnTastSide(); }")
}

// WaitUntilClosed waits until the corresponding CCA window is closed.
func (a *AppWindow) WaitUntilClosed(ctx context.Context) error {
	return a.jsObj.Call(ctx, nil, "function() { return this.waitUntilClosed(); }")
}

// ClosingItself checks if CCA intends to close itself.
func (a *AppWindow) ClosingItself(ctx context.Context) (bool, error) {
	var closing bool
	if err := a.jsObj.Call(ctx, &closing, "function() { return this.isClosingItself(); }"); err != nil {
		return false, err
	}
	return closing, nil
}

// Perfs returns the collected perf entries.
func (a *AppWindow) Perfs(ctx context.Context) ([]PerfEntry, error) {
	var entries []PerfEntry
	if err := a.jsObj.Call(ctx, &entries, "function() { return this.getPerfs(); }"); err != nil {
		return nil, err
	}
	return entries, nil
}

// Errors returns the collected error information.
func (a *AppWindow) Errors(ctx context.Context) ([]ErrorInfo, error) {
	var errorInfos []ErrorInfo
	if err := a.jsObj.Call(ctx, &errorInfos, "function() { return this.getErrors(); }"); err != nil {
		return nil, err
	}
	return errorInfos, nil
}

// Release releases the app window instance.
func (a *AppWindow) Release(ctx context.Context) error {
	err := a.jsObj.Release(ctx)
	a.jsObj = nil
	return err
}
