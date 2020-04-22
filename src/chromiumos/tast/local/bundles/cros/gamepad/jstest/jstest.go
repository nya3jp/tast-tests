// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package jstest contains utilities to run device tests using the
// chrome JS controller API.
package jstest

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/uhid"
	"chromiumos/tast/testing"
)

// DeviceInfoSet is used to determine when the get report requests by
// the kernel are done.
var DeviceInfoSet = false

// Gamepad runs a javascript test for the given device. It compares
// the buttons listed in expectedButtons with the ones produced by the
// recording.
func Gamepad(ctx context.Context, s *testing.State, d *uhid.Device, replayPath, buttonMappings string, expectedButtons []string) {
	for !DeviceInfoSet {
		status, err := d.Dispatch(ctx)
		if err != nil {
			s.Fatal("Failed during kernel communication: ", err)
		}
		// If no event was caught the kernel does not require a
		// GetReportReply, we can proceed with the test.
		if status == uhid.StatusNoEvent {
			s.Log("No get report requests made by kernel")
			break
		}
	}
	s.Log("Kernel information requests done")

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	conn, err := waitForScriptReady(ctx, server, buttonMappings)
	if err != nil {
		s.Fatal("Failed creating connection: ", err)
	}
	s.Log("Script ready")
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	s.Log("Starting replay and js test")
	js := make(chan error)
	go runJavascriptTest(ctx, conn, expectedButtons, js)
	if err := replayDevice(ctx, d, replayPath); err != nil {
		s.Fatal("replay failed: ", err)
	}
	d.Close()

	if err, ok := <-js; ok {
		s.Fatal("Javascript test failed: ", err)
	}
}

// CreateDevice creates the dualshock3 device with the proper uniq and
// sets the correct event handlers for kernel events.
func CreateDevice(ctx context.Context, path string) (*uhid.Device, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var d *uhid.Device
	if d, err = uhid.NewDeviceFromRecording(ctx, file); err != nil {
		return nil, err
	}
	if err = d.NewKernelDevice(ctx); err != nil {
		return nil, err
	}
	return d, nil
}

func replayDevice(ctx context.Context, d *uhid.Device, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	if err := d.Replay(ctx, file); err != nil {
		return err
	}
	return nil
}

func waitForScriptReady(ctx context.Context, server *httptest.Server, mappings string) (*chrome.Conn, error) {
	cr, err := chrome.New(ctx)
	if err != nil {
		return nil, err
	}

	conn, err := cr.NewConn(ctx, path.Join(server.URL, "ds3Replay.html"))
	if err != nil {
		return nil, err
	}

	if err := conn.WaitForExpr(ctx, "scriptReady"); err != nil {
		return nil, errors.Wrap(err, "failed waiting for script to be ready")
	}

	call := fmt.Sprintf("setMappings(%s)", mappings)
	if err := conn.WaitForExpr(ctx, call); err != nil {
		return nil, errors.Wrap(err, "failed setting button mappings")
	}

	return conn, nil
}

func runJavascriptTest(ctx context.Context, conn *chrome.Conn, expected []string, c chan error) {
	if err := conn.WaitForExpr(ctx, "gamepadDisconnected"); err != nil {
		c <- err
		return
	}

	var actual []string
	if err := conn.Eval(ctx, "getResults()", &actual); err != nil {
		c <- err
		return
	}

	if err := compareButtonSequence(expected, actual); err != nil {
		c <- err
		return
	}

	close(c)
}

func compareButtonSequence(expected, actual []string) error {
	if len(expected) != len(actual) {
		return errors.Errorf("expected button array length and actual button array lengths differ: got %d, wanted %d", len(actual), len(expected))
	}
	for i, v := range expected {
		if v != actual[i] {
			return errors.Errorf("Button discrepancy at index %d: got %s, wanted %s", i, actual[i], v)
		}
	}
	return nil
}
