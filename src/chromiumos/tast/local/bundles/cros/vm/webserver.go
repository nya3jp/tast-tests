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
		Func:         Webserver,
		Desc:         "Checks that a webserver can start and is accessible from Chrome",
		Attr:         []string{"informational"},
		Timeout:      300 * time.Second,
		SoftwareDeps: []string{"chrome_login", "vm_host"},
	})
}

func Webserver(s *testing.State) {
	const (
		defaultContainerUrl = "http://localhost:8000"
		expectedWebContent  = "nothing but the web"
	)

	defer faillog.SaveIfError(s)

	cr, err := chrome.New(s.Context())
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(s.Context())

	err = vm.SetUpComponent(s.Context(), vm.StagingComponent)
	if err != nil {
		s.Fatal("Failed to set up component: ", err)
	}

	cont, err := vm.CreateDefaultContainer(s.Context(), cr.User(), vm.StagingImageServer)
	if err != nil {
		s.Fatal("Failed to set up default container: ", err)
	}
	defer vm.StopConcierge(s.Context())

	cmd := cont.Command(s.Context(),
		fmt.Sprintf("echo '%s' > ~/index.html", expectedWebContent))
	if err = cmd.Run(); err != nil {
		cmd.DumpLog(s.Context())
		s.Fatal("Failed to add test index.html: ", err)
	}

	cmd = cont.Command(s.Context(), "python2", "-m", "SimpleHTTPServer")
	if err = cmd.Start(); err != nil {
		cmd.DumpLog(s.Context())
		s.Fatal("Failed to run python2: ", err)
	}

	conn, err := cr.NewConn(s.Context(), "")
	if err != nil {
		s.Fatal("Creating renderer failed: ", err)
	}
	defer conn.Close()

	if err = conn.Navigate(s.Context(), defaultContainerUrl); err != nil {
		s.Fatalf("Navigating to %q failed: %v", defaultContainerUrl, err)
	}
	var actual string
	if err = conn.Eval(s.Context(), "document.documentElement.innerText", &actual); err != nil {
		s.Fatal("Getting page content failed: ", err)
	}
	if !strings.HasPrefix(actual, expectedWebContent) {
		s.Fatalf("Expected page content %q, got %q", expectedWebContent, actual)
	}
}
