// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ui allows interacting with Android apps by Android UI Automator API.
// We use android-uiautomator-server, a JSON-RPC server running as an Android app,
// to invoke UI Automator methods remotely.
package ui

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

const (
	// StartTimeout is the timeout of NewDevice.
	StartTimeout = 60 * time.Second

	// host is the hard-coded IP address of Android container and fixed port of
	// the UI automator server.
	host = "100.115.92.2:9008"

	serverPackage  = "com.github.uiautomator.test"
	serverActivity = "androidx.test.runner.AndroidJUnitRunner"
)

var apkPaths = []string{
	"/usr/local/share/android-uiautomator-server/app-uiautomator.apk",
	"/usr/local/share/android-uiautomator-server/app-uiautomator-test.apk",
}

// Device provides access to state information about the Android system.
//
// Close must be called to clean up resources when a test is over.
//
// This object corresponds to UiDevice in UI Automator API:
// https://developer.android.com/reference/android/support/test/uiautomator/UiDevice
type Device struct {
	a     *arc.ARC
	sp    *testexec.Cmd // Server process
	debug bool
}

type jsonRPCRequest struct {
	Version string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	ID      int           `json:"id"`
	Params  []interface{} `json:"params,omitempty"`
}

type jsonRPCError struct {
	Message string `json:"message"`
}

type jsonRPCResponse struct {
	Version string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result"`
	Error   *jsonRPCError   `json:"error"`
}

// DeviceInfo holds information about the device. See:
// https://github.com/xiaocong/android-uiautomator-server/blob/master/app/src/androidTest/java/com/github/uiautomator/stub/DeviceInfo.java
type DeviceInfo struct {
	CurrentPackagename string `json:"currentPackagename"`
	DisplayWidth       int    `json:"displayWidth"`
	DisplayHeight      int    `json:"displayHeight"`
	DisplayRotation    int    `json:"displayRotation"`
	DisplaySizeDpX     int    `json:"displaySizeDpX"`
	DisplaySizeDpY     int    `json:"displaySizeDpY"`
	ProductName        string `json:"productName"`
	NaturalOrientation bool   `json:"naturalOrientation"`
	ScreenOn           bool   `json:"screenOn"`
	SDKInt             int    `json:"sdkInt"`
}

// NewDevice creates a Device object by starting and connecting to UI Automator server.
//
// Close must be called to clean up resources when a test is over.
func NewDevice(ctx context.Context, a *arc.ARC) (*Device, error) {
	ictx, cancel := context.WithTimeout(ctx, StartTimeout)
	defer cancel()

	testing.ContextLog(ctx, "Starting UI Automator server")

	if err := installServer(ictx, a); err != nil {
		return nil, err
	}

	sp := a.Command(ctx, "am", "instrument", "-w", serverPackage+"/"+serverActivity)
	if err := sp.Start(); err != nil {
		return nil, errors.Wrap(err, "failed starting UI Automator server")
	}

	s := &Device{a, sp, false}

	if err := s.waitServer(ictx); err != nil {
		s.Close()
		return nil, errors.Wrap(err, "UI Automator server did not come up")
	}

	return s, nil
}

// installServer installs UI Automator server to Android system.
func installServer(ctx context.Context, a *arc.ARC) error {
	for _, p := range apkPaths {
		if err := a.Install(ctx, p); err != nil {
			return errors.Wrapf(err, "failed installing %s", p)
		}
	}
	return nil
}

// waitServer waits for UI Automator server to come up.
func (d *Device) waitServer(ctx context.Context) error {
	for {
		if ok := func() bool {
			ctx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
			defer cancel()
			var res string
			return d.call(ctx, "ping", &res) == nil && res == "pong"
		}(); ok {
			break
		}

		select {
		case <-time.After(100 * time.Millisecond):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

// EnableDebug enables verbose RPC logging.
func (d *Device) EnableDebug() {
	d.debug = true
}

// Close releases resources associated with d.
func (d *Device) Close() error {
	d.sp.Kill()
	return d.sp.Wait()
}

// call calls a remote server method by JSON-RPC.
// method is a method name.
// out is a variable to store a returned result. If it is nil, results are discarded.
// params is a list of parameters to the remote method.
func (d *Device) call(ctx context.Context, method string, out interface{}, params ...interface{}) error {
	// Prepare the request.
	req, err := http.NewRequest("POST", "http://"+host+"/jsonrpc/0", nil)
	if err != nil {
		return errors.Wrapf(err, "%s: failed initializing request", method)
	}
	req = req.WithContext(ctx)

	reqData := jsonRPCRequest{
		Version: "2.0",
		Method:  method,
		Params:  params,
	}
	reqBody, err := json.Marshal(&reqData)
	if err != nil {
		return errors.Wrapf(err, "%s: failed marshaling request", method)
	}
	req.Body = ioutil.NopCloser(bytes.NewReader(reqBody))
	req.ContentLength = int64(len(reqBody))
	req.Header.Add("Content-Type", "application/json")

	if d.debug {
		testing.ContextLog(ctx, "-> ", string(reqBody))
	}

	// Send the request.
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return errors.Wrap(err, method)
	}
	defer res.Body.Close()

	// Status should be OK.
	if res.StatusCode != 200 {
		return errors.Errorf("%s: got status %d", method, res.StatusCode)
	}

	// Read and parse the response.
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return errors.Wrapf(err, "%s: failed reading response", method)
	}

	if d.debug {
		testing.ContextLog(ctx, "<- ", string(resBody))
	}

	var resData jsonRPCResponse
	if err := json.Unmarshal(resBody, &resData); err != nil {
		return errors.Wrapf(err, "%s: failed unmarshaling response", method)
	}

	// Check an error.
	if resData.Error != nil {
		return errors.Errorf("%s: %s", method, resData.Error.Message)
	}

	// If the caller does not need results, we can return now.
	if out == nil {
		return nil
	}

	// Parse the result.
	if len(resData.Result) == 0 {
		return errors.Errorf("%s: missing result", method)
	}
	if err := json.Unmarshal([]byte(resData.Result), out); err != nil {
		testing.ContextLogf(ctx, "Failed unmarshaling to %T: %q", out, string(resData.Result))
		return errors.Wrapf(err, "%s: failed unmarshaling result", method)
	}
	return nil
}

// WaitForIdle waits for the current application to idle.
// This method corresponds to UiDevice.waitForIdle(long).
// https://developer.android.com/reference/android/support/test/uiautomator/UiDevice.html#waitForIdle(long)
func (d *Device) WaitForIdle(ctx context.Context, timeout time.Duration) error {
	return d.call(ctx, "waitForIdle", nil, timeout/time.Millisecond)
}

// WaitForWindowUpdate waits for a window content update event to occur.
// If a package name for the window is specified, but the current window does not have the same package name,
// the function returns eventOcurred=false, but no error is generated.
// Returns true if a window update occurred, false if timeout has elapsed or if the current window does not have the specified package name.
// This method corresponds to UiDevice.waitForWindowUpdate(string, long).
// https://developer.android.com/reference/android/support/test/uiautomator/UiDevice.html#waitforwindowupdate
func (d *Device) WaitForWindowUpdate(ctx context.Context, packageName string, timeout time.Duration) (eventOcurred bool, err error) {
	if err := d.call(ctx, "waitForWindowUpdate", &eventOcurred, packageName, timeout/time.Millisecond); err != nil {
		return false, err
	}
	return eventOcurred, nil
}

// PressKeyCode simulates a short press using a key code.
// keyCode is the key code. metaState represents the meta keys. Each bit represents a pressed meta key.
// This method corresponds to UiDevice.pressKeyCode(int, int)
// https://developer.android.com/reference/android/support/test/uiautomator/UiDevice#presskeycode
func (d *Device) PressKeyCode(ctx context.Context, keyCode KeyCode, metaState MetaState) error {
	var success bool
	if err := d.call(ctx, "pressKeyCode", &success, keyCode, metaState); err != nil {
		return err
	}
	if !success {
		return errors.Errorf("failed to press keycode=%d, meta=%#x", keyCode, metaState)
	}
	return nil
}

// GetInfo returns the device info.
// This method corresponds to the com.github.uiautomator.stub.getDeviceInfo().
// https://github.com/xiaocong/android-uiautomator-server/blob/master/app/src/androidTest/java/com/github/uiautomator/stub/DeviceInfo.java
func (d *Device) GetInfo(ctx context.Context) (*DeviceInfo, error) {
	var info DeviceInfo
	if err := d.call(ctx, "deviceInfo", &info); err != nil {
		return nil, err
	}
	return &info, nil
}
