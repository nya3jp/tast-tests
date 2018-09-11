// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package webrtc provides common code for video.WebRTC* tests.
package webrtc

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/faillog"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// DataFiles returns a list of required files that tests that use this package
// should include in their Data fields.
func DataFiles() []string {
	return []string{
		"blackframe.js",
		"ssim.js",
	}
}

// isVM returns true if the test is running under QEMU.
func isVM(s *testing.State) bool {
	const path = "/sys/devices/virtual/dmi/id/sys_vendor"
	content, err := ioutil.ReadFile(path)

	if err != nil {
		return false
	}

	vendor := strings.TrimSpace(string(content))
	s.Logf("%s : %s", path, vendor)

	return vendor == "QEMU"
}

// loadVivid loads the "vivid" kernel module, which emulates a video capture device.
func loadVivid(ctx context.Context) error {
	cmd := testexec.CommandContext(ctx, "modprobe", "vivid", "n_devs=1", "node_types=0x1")

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(output)))
	}

	return nil
}

// unloadVivid unloads the "vivid" kernel module.
func unloadVivid(ctx context.Context) error {
	// Use Poll instead of executing modprobe once, because modprobe may fail
	// if it is called before the device is completely released from camera HAL.
	return testing.Poll(ctx, func(ctx context.Context) error {
		cmd := testexec.CommandContext(ctx, "modprobe", "-r", "vivid")

		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(output)))
		}
		return nil
	}, nil)
}

// RunTest checks if the given WebRTC tests work correctly.
// htmlName is a filename of an HTML file in data directory.
// entryPoint is a JavaScript expression that starts the test there.
func RunTest(s *testing.State, htmlName, entryPoint string) {
	defer faillog.SaveIfError(s)

	ctx := s.Context()

	if isVM(s) {
		s.Log("Loading vivid")
		if err := loadVivid(ctx); err != nil {
			s.Fatal("Failed to load vivid: ", err)
		}
		defer func() {
			s.Log("Unloading vivid")
			if err := unloadVivid(ctx); err != nil {
				s.Fatal("Failed to unload vivid: ", err)
			}
		}()
	}

	cr, err := chrome.New(ctx, chrome.ExtraArgs([]string{"--use-fake-ui-for-media-stream"}))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	conn, err := cr.NewConn(s.Context(), server.URL+"/"+htmlName)
	if err != nil {
		s.Fatal("Creating renderer failed: ", err)
	}
	defer conn.Close()

	if err := conn.WaitForExpr(ctx, "scriptReady"); err != nil {
		s.Fatal("Timed out waiting for scripts ready: ", err)
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
