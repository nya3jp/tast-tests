// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package subtest

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

type size struct {
	W int `json:"width"`
	H int `json:"height"`
}

// AppDisplayDensity executes a X11 or wayland test application directly from the command
// line in the terminal twice with default and low display density respectively
// and verifies that it renders the window bigger in low display density.
func AppDisplayDensity(ctx context.Context, s *testing.State, tconn *chrome.Conn,
	cont *vm.Container, name, command string) {
	s.Log("Executing test app from terminal launch for ", name)
	commandWidth := fmt.Sprintf("--width=%d", 100)
	commandHeight := fmt.Sprintf("--height=%d", 100)
	commandTitle := "--title=" + name
	cmd := cont.Command(ctx, command, commandWidth, commandHeight, commandTitle)
	s.Logf("Running %q", strings.Join(cmd.Args, " "))
	if err := cmd.Start(); err != nil {
		s.Errorf("Failed launching %q: %v", strings.Join(cmd.Args, " "), err)
		cmd.DumpLog(ctx)
		return
	}

	sizeHighDensity, err := getWindowSizeWithPoll(ctx, tconn, name)
	cmd.Kill()
	cmd.Wait()
	if err != nil {
		s.Errorf("Failed getting window %q size: %v", name, err)
		return
	}
	s.Logf("window %q size is %v", name, sizeHighDensity)

	name += "_low_density"
	commandTitle = "--title=" + name
	subCommandArgs := []string{"DISPLAY=${DISPLAY_LOW_DENSITY}", "WAYLAND_DISPLAY=${WAYLAND_DISPLAY_LOW_DENSITY}", command, commandWidth, commandHeight, commandTitle}
	subCommand := strings.Join(subCommandArgs, " ")
	cmd = cont.Command(ctx, "sh", "-c", subCommand)
	s.Logf("Running %q", strings.Join(cmd.Args, " "))
	if err := cmd.Start(); err != nil {
		s.Errorf("Failed launching %q: %v", strings.Join(cmd.Args, " "), err)
		cmd.DumpLog(ctx)
		return
	}

	sizeLowDensity, err := getWindowSizeWithPoll(ctx, tconn, name)
	cmd.Kill()
	cmd.Wait()
	if err != nil {
		s.Errorf("Failed getting window %q size: %v", name, err)
		return
	}

	if sizeHighDensity.W >= sizeLowDensity.W || sizeHighDensity.H >= sizeLowDensity.H {
		s.Errorf("High density size %v is not smaller than low density size %v", sizeHighDensity, sizeLowDensity)
	}
}

// getWindowSizeWithPoll returns the the width and the height of the window in pixels
// with polling to wait for asynchronous rendering on the DUT.
func getWindowSizeWithPoll(ctx context.Context, tconn *chrome.Conn, name string) (sz size, err error) {
	// Allow up to 10 seconds for the target screen to render.
	err = testing.Poll(ctx, func(ctx context.Context) error {
		var err error
		sz, err = getWindowSize(ctx, tconn, name)
		return err
	}, &testing.PollOptions{Timeout: 10 * time.Second})
	return sz, err
}

// getWindowSize returns the the width and the height of the window in pixels.
func getWindowSize(ctx context.Context, tconn *chrome.Conn, name string) (sz size, err error) {
	expr := fmt.Sprintf(
		`new Promise((resolve, reject) => {
			chrome.automation.getDesktop(root => {
				const appWindow = root.find({ attributes: { name: %q}});
				if (!appWindow) {
					reject("Failed to locate the app window");
				}
				const view = appWindow.find({ attributes: { className: 'ui/views/window/ClientView'}});
				if (!view) {
					reject("Failed to find client view");
				}
				resolve(view.location);
			})
		})`, name)
	err = tconn.EvalPromise(ctx, expr, &sz)
	return sz, err
}
