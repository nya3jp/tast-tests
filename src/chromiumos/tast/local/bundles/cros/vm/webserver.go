// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"strings"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/dbusutil"
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
	// Start listening for a "started" SessionStateChanged D-Bus signal from session_manager.
	sw, err := dbusutil.NewSignalWatcherForSystemBus(s.Context(), dbusutil.MatchSpec{
		Type:      "signal",
		Path:      dbusutil.SessionManagerPath,
		Interface: dbusutil.SessionManagerInterface,
		Member:    "SessionStateChanged",
		Arg0:      "started",
	})
	if err != nil {
		s.Fatal("Failed to watch for D-Bus signals: ", err)
	}
	defer sw.Close()

	cr, err := chrome.New(s.Context())
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(s.Context())

	s.Log("Waiting for SessionStateChanged \"started\" D-Bus signal from session_manager")
	select {
	case <-sw.Signals:
		s.Log("Got SessionStateChanged signal")
	case <-s.Context().Done():
		s.Fatal("Didn't get SessionStateChanged signal: ", s.Context().Err())
	}

	// TODO(smbarber): Uncomment when tremplin is in live component. For local testing only.
	// err = vm.SetUpComponent(s.Context(), vm.StagingComponent)
	// if err != nil {
	// 	s.Fatal("Failed to set up component: ", err)
	// }

	_, _, c, err := vm.NewDefaultContainer(s.Context(), cr.User(), vm.LiveImageServer)
	if err != nil {
		s.Fatal("Failed to set up test fixture: ", err)
	}

	cmd := c.Command(s.Context(), "sudo", "apt-get", "-y", "install", "nginx-light")
	_, err = cmd.Output()
	if err != nil {
		cmd.DumpLog(s.Context())
		s.Error("Failed to install nginx: ", err)
	}

	conn, err := cr.NewConn(s.Context(), "")
	if err != nil {
		s.Fatal("Creating renderer failed: ", err)
	}
	defer conn.Close()

	if err = conn.Navigate(s.Context(), "http://penguin.linux.test"); err != nil {
		s.Fatalf("Navigating to linuxhost failed: %v", err)
	}
	var actual string
	if err = conn.Eval(s.Context(), "document.documentElement.innerText", &actual); err != nil {
		s.Fatal("Getting page content failed: ", err)
	}
	const expected = "Welcome to nginx!"
	if !strings.HasPrefix(actual, expected) {
		s.Fatalf("Expected page content %q, got %q", expected, actual)
	}
}
