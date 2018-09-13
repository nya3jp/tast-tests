// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package subtest

import (
	"bufio"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

// Webserver starts an HTTP server in the container and verifies that
// Chrome (outside of the container) is able to access it.
func Webserver(s *testing.State, cr *chrome.Chrome, cont *vm.Container) {
	s.Log("Executing Webserver test")
	ctx := s.Context()

	const expectedWebContent = "nothing but the web"

	cmd := cont.Command(ctx, "sh", "-c",
		fmt.Sprintf("echo '%s' > ~/index.html", expectedWebContent))
	if err := cmd.Run(); err != nil {
		cmd.DumpLog(ctx)
		s.Error("webserver: Failed to add test index.html: ", err)
		return
	}
	cmd = cont.Command(ctx, "python2", "-m", "SimpleHTTPServer")

	// Start a goroutine that reads lines from the python exec and writes them to a channel.
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		s.Error("webserver: Failed to get stdout for python exec: ", err)
		return
	}
	ch := make(chan string)
	go func() {
		sc := bufio.NewScanner(stdout)
		for sc.Scan() {
			ch <- sc.Text()
		}
		close(ch)
	}()

	// waitForOutput waits until a line matched by re is written to ch, python's stdout is closed,
	// or the deadline is reached. It returns the full line that was matched.
	waitForOutput := func(re *regexp.Regexp) (string, error) {
		for {
			select {
			case line, more := <-ch:
				if !more {
					return "", errors.New("eof")
				}
				if re.MatchString(line) {
					return line, nil
				}
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}
	}

	if err := cmd.Start(); err != nil {
		cmd.DumpLog(ctx)
		s.Error("webserver: Failed to run python2", err)
		return
	}
	defer cmd.Wait()
	defer cmd.Kill()

	testing.ContextLog(ctx, "Waiting for python webserver to start up")
	_, err = waitForOutput(regexp.MustCompile("Serving HTTP.*"))
	if err != nil {
		s.Error("webserver: Error waiting for python webserver to start up: ", err)
	}
	testing.ContextLog(ctx, "Python webserver startup completed")

	conn, err := cr.NewConn(ctx, "")
	if err != nil {
		s.Error("webserver: Creating renderer failed: ", err)
		return
	}
	defer conn.Close()

	checkNavigation := func(url string) {
		if err = conn.Navigate(ctx, url); err != nil {
			s.Errorf("webserver: Navigating to %q failed: %v", url, err)
			return
		}
		var actual string
		if err = conn.Eval(ctx, "document.documentElement.innerText", &actual); err != nil {
			s.Error("webserver: Getting page content failed: ", err)
			return
		}
		if !strings.HasPrefix(actual, expectedWebContent) {
			s.Errorf("webserver: Expected page content %q, got %q", expectedWebContent, actual)
			return
		}
	}

	containerUrls := []string{"http://penguin.linux.test:8000", "http://localhost:8000"}
	for _, url := range containerUrls {
		checkNavigation(url)
	}
}
