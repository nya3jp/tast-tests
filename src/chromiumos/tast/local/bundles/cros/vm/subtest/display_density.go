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

// AppDisplayDensity executes a X11 or wayland test application directly from the command
// line in the terminal twice with default and low display density respectively
// and verifies that it renders the window bigger in low display density.
func AppDisplayDensity(ctx context.Context, s *testing.State, tconn *chrome.Conn,
	cont *vm.Container, name, command string) {
	s.Log("Executing test app from terminal launch for ", name)
	commandWidth := fmt.Sprintf("--width=%d", 100)
	commandHeight := fmt.Sprintf("--height=%d", 100)
	commandTitle := fmt.Sprintf("--title=%s", name)
	cmd := cont.Command(ctx, command, commandWidth, commandHeight, commandTitle)
	s.Logf("Running: %q", strings.Join(cmd.Args, " "))
	if err := cmd.Start(); err != nil {
		defer cmd.DumpLog(ctx)
		s.Errorf("Failed launching %v: %v", strings.Join(cmd.Args, " "), err)
		return
	}

	wH, hH, err := getWindowSizeWithPoll(ctx, tconn, name, s)
	if err != nil {
		s.Errorf("Failed getting window %v size: %v", name, err)
		return
	}
	cmd.Kill()
	cmd.Wait()

	name += "_low_density"
	commandTitle = fmt.Sprintf("--title=%s", name)
	subCommandArgs := []string{"DISPLAY=${DISPLAY_LOW_DENSITY}", "WAYLAND_DISPLAY=${WAYLAND_DISPLAY_LOW_DENSITY}", command, commandWidth, commandHeight, commandTitle}
	subCommand := strings.Join(subCommandArgs, " ")
	cmd = cont.Command(ctx, "sh", "-c", subCommand)
	s.Logf("Running: %q", strings.Join(cmd.Args, " "))
	if err := cmd.Start(); err != nil {
		defer cmd.DumpLog(ctx)
		s.Errorf("Failed launching %v: %v", strings.Join(cmd.Args, " "), err)
		return
	}

	wL, hL, err := getWindowSizeWithPoll(ctx, tconn, name, s)
	if err != nil {
		s.Errorf("Failed getting window %v size: %v", name, err)
		return
	}
	cmd.Kill()
	cmd.Wait()

	if wH >= wL || hH >= hL {
		s.Errorf("High density size is not smaller than low density size")
	}
}

// getWindowSizeWithPoll returns the the width and the height of the window in pixels
// with polling to wait for asynchronous redering on the DUT.
func getWindowSizeWithPoll(ctx context.Context, tconn *chrome.Conn, name string, s *testing.State) (int, int, error) {
	var wr int
	var hr int
	// Allow up to 10 seconds for the target screen to render.
	err := testing.Poll(ctx, func(ctx context.Context) error {
		var erri error
		if wr, hr, erri = getWindowSize(ctx, tconn, name, s); erri != nil {
			return erri
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second})
	if err != nil {
		return 0, 0, err
	}
	return wr, hr, nil
}

// getWindowSize returns the the width and the height of the window in pixels.
func getWindowSize(ctx context.Context, tconn *chrome.Conn, name string, st *testing.State) (int, int, error) {
	expr := fmt.Sprintf(
		`new Promise((resolve, reject) => {
			chrome.automation.getDesktop(root => {
				const appWindow = root.find({ attributes: { name: '%v'}});
				if (!appWindow) {
					reject("Failed to locate the app window");
				}
				const view = appWindow.find({ attributes: { className: 'ui/views/window/ClientView'}});
				if (!view) {
					reject("Failed to find client view.");
				}
				resolve(view.location);
			})
		})`, name)
	location := make(map[string]int)
	if err := tconn.EvalPromise(ctx, expr, &location); err != nil {
		return 0, 0, err
	}
	return location["width"], location["height"], nil
}
