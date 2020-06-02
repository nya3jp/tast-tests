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

// KernelCommunicationDone is used to determine when the get report requests by
// the kernel are done.
var KernelCommunicationDone = false

// Gamepad runs a javascript test for the given device. It compares
// the buttons listed in expectedButtons with the ones produced by the
// recording.
func Gamepad(ctx context.Context, s *testing.State, d *uhid.Device, replayPath, buttonMappings string, expectedButtons []string) {
	for !KernelCommunicationDone {
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
	defer server.Close()
	conn, err := prepareScript(ctx, server, buttonMappings)
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
		s.Fatal("Replay failed: ", err)
	}
	if err := d.Close(); err != nil {
		s.Fatal("Failed to destroy controller: ", err)
	}
	s.Log("Destroyed device")

	if err, ok := <-js; ok {
		s.Fatal("JavaScript test failed: ", err)
	}
}

// CreateDevice creates the device in the given recording and
// initializes it.
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

// replayDevice replays on d the events in the file in path. Returns
// error if opening the file failed or if the replay failed.
func replayDevice(ctx context.Context, d *uhid.Device, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return d.Replay(ctx, file)
}

// prepareScript returns the connection created from the url for
// ds3_replay.html and sets the given controller mappings.
func prepareScript(ctx context.Context, server *httptest.Server, mappings string) (*chrome.Conn, error) {
	cr, err := chrome.New(ctx)
	if err != nil {
		return nil, err
	}

	conn, err := cr.NewConn(ctx, path.Join(server.URL, "replay.html"))
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

// runJavascriptTest waits for the gamepad disconnected event
// (triggered by a call to uhid.Device.Close). After this it compares
// the button sequence gathered by the test with the expected button
// sequence. If at any point there's an error it will be written to
// the channel and the function will return.
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

// compareButtonSequence compares the expected button sequence with
// the actual one and returns and error if there's a mismatch.
func compareButtonSequence(expected, actual []string) error {
	if len(expected) != len(actual) {
		return errors.Errorf("expected button array length and actual button array lengths differ: got %d, want %d", len(actual), len(expected))
	}
	for i, v := range expected {
		if v != actual[i] {
			return errors.Errorf("button discrepancy at index %d: got %s, want %s", i, actual[i], v)
		}
	}
	return nil
}
