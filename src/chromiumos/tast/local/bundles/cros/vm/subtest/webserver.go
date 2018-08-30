// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package subtest

import (
	"fmt"
	"strings"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

// Webserver starts an HTTP server in the container and verifies that
// Chrome (outside of the container) is able to access it.
func Webserver(s *testing.State, cr *chrome.Chrome, cont *vm.Container) {
	s.Log("Executing Webserver test")

	const expectedWebContent = "nothing but the web"

	cmd := cont.Command(s.Context(), "sh", "-c",
		fmt.Sprintf("echo '%s' > ~/index.html", expectedWebContent))
	if err := cmd.Run(); err != nil {
		cmd.DumpLog(s.Context())
		s.Error("webserver: Failed to add test index.html: ", err)
		return
	}
	cmd = cont.Command(s.Context(), "python2", "-m", "SimpleHTTPServer")
	if err := cmd.Start(); err != nil {
		cmd.DumpLog(s.Context())
		s.Error("webserver: Failed to run python2", err)
		return
	}
	defer cmd.Wait()
	defer cmd.Kill()

	conn, err := cr.NewConn(s.Context(), "")
	if err != nil {
		s.Error("webserver: Creating renderer failed: ", err)
		return
	}
	defer conn.Close()

	checkNavigation := func(url string) {
		if err = conn.Navigate(s.Context(), url); err != nil {
			s.Errorf("webserver: Navigating to %q failed: %v", url, err)
			return
		}
		var actual string
		if err = conn.Eval(s.Context(), "document.documentElement.innerText", &actual); err != nil {
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
