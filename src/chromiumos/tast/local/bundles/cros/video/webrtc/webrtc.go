// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package webrtc provides common codes for video.WebRtc* tests.
package webrtc

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/local/bundles/cros/video/play"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/faillog"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// RunTest checks if the given WebRTC tests work correctly
func RunTest(s *testing.State, htmlName string, entryPoint string) {
	defer faillog.SaveIfError(s)

	ctx := s.Context()

	cr, err := chrome.New(ctx, chrome.ExtraArgs([]string{"--use-fake-ui-for-media-stream"}))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	server := httptest.NewServer(http.FileServer((*play.DataDir)(s)))
	defer server.Close()

	conn, err := cr.NewConn(s.Context(), server.URL+"/"+htmlName)
	if err != nil {
		s.Fatal("Creating renderer failed: ", err)
	}
	defer conn.Close()

	if err := conn.WaitForExpr(ctx, "scriptReady"); err != nil {
		s.Fatal("Timed out waiting for scripts ready: ", err)
	}

	// TODO(crbug.com/871185):
	// loadVivid(s) should be called before chrome.New(...) is called
	if isVM(s) {
		loadVivid(s)
		defer unloadVivid(s)
	}

	if err := conn.WaitForExpr(ctx, "checkVideoInput()"); err != nil {
		s.Fatal("Timed out waiting for video device to be available: ", err)
	}

	if err := conn.Exec(ctx, entryPoint); err != nil {
		s.Fatal("Failed to start test: ", err)
	}

	if err := conn.WaitForExpr(ctx, "isTestDone"); err != nil {
		s.Fatal("Timed out waiting for test completed: ", err)
	}
}

// Check if the test is running on QEMU
func isVM(s *testing.State) bool {
	vendor, err := ioutil.ReadFile("/sys/devices/virtual/dmi/id/sys_vendor")

	if err != nil {
		// if sys_vendor file is not provided there,
		// the device is not a VM
		return false
	}

	s.Log("/sys/devices/virtual/dmi/id/sys_vendor : ", string(vendor))

	return string(vendor) == "QEMU\n"
}

// Load vivid module
func loadVivid(s *testing.State) {
	defer faillog.SaveIfError(s)

	s.Log("Load vivid")

	ctx := s.Context()

	cmd := testexec.CommandContext(ctx, "sudo", "modprobe", "vivid", "n_devs=1", "node_types=0x1")
	if output, err := cmd.CombinedOutput(); err != nil {
		s.Fatal("Failed to load vivid: ", string(output), err)
	}
}

// Unload vivid module
func unloadVivid(s *testing.State) {
	defer faillog.SaveIfError(s)

	ctx := s.Context()

	s.Log("Unload vivid")

	cmd := testexec.CommandContext(ctx, "sudo", "modprobe", "-r", "vivid")

	// Wait a second for vivid released
	time.Sleep(time.Second)

	if output, err := cmd.CombinedOutput(); err != nil {
		s.Error("Failed to unload vivid: ", string(output), err)
	}
}
