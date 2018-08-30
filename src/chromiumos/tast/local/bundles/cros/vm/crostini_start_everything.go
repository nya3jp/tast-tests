// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/faillog"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CrostiniStartEverything,
		Desc:         "Tests Termina VM startup, container startup and other Crostini functionality",
		Attr:         []string{"informational"},
		Timeout:      300 * time.Second,
		SoftwareDeps: []string{"chrome_login", "vm_host"},
	})
}

func CrostiniStartEverything(s *testing.State) {
	defer faillog.SaveIfError(s)

	cr, err := chrome.New(s.Context())
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(s.Context())

	s.Log("Setting up component ", vm.StagingComponent)
	err = vm.SetUpComponent(s.Context(), vm.StagingComponent)
	if err != nil {
		s.Fatal("Failed to set up component: ", err)
	}

	s.Log("Creating default container")
	cont, err := vm.CreateDefaultContainer(s.Context(), cr.User(), vm.StagingImageServer)
	if err != nil {
		s.Fatal("Failed to set up default container: ", err)
	}
	defer vm.StopConcierge(s.Context())

	s.Log("Verifying pwd command works")
	cmd := cont.Command(s.Context(), "pwd")
	if err = cmd.Run(); err != nil {
		cmd.DumpLog(s.Context())
		s.Fatal("Failed to run pwd: ", err)
	}

	// The VM and container have started up so we can now execute all of the other
	// Crostini tests. We need to be careful about this because we are going to be
	// testing multiple things in one test. This should be done so that no tests
	// have any known dependency on prior tests. If we hit a conflict at some
	// point then we will need to add functionality to save the VM/container image
	// at this point and then stop the VM/container and restore that image so we
	// can have a clean VM/container to start from again. Failures should not be
	// fatal so that all tests can get executed.
	s.Log("Executing webserver test")
	webserver(s, cr, cont)
}

func webserver(s *testing.State, cr *chrome.Chrome, cont *vm.Container) {
	const (
		expectedWebContent = "nothing but the web"
	)

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
