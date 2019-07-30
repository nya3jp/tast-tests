// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"chromiumos/tast/local/crostini"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Webserver,
		Desc:         "Runs a webserver in the container, and confirms that the host can connect to it",
		Contacts:     []string{"smbarber@chromium.org", "cros-containers-dev@google.com"},
		Attr:         []string{"informational"},
		Timeout:      7 * time.Minute,
		Data:         []string{crostini.ImageArtifact},
		Pre:          crostini.StartedByArtifact(),
		SoftwareDeps: []string{"chrome", "vm_host"},
	})
}

func Webserver(ctx context.Context, s *testing.State) {
	pre := s.PreValue().(crostini.PreData)
	cr := pre.Chrome
	cont := pre.Container

	const expectedWebContent = "nothing but the web"

	cmd := cont.Command(ctx, "sh", "-c",
		fmt.Sprintf("echo '%s' > ~/index.html", expectedWebContent))
	if err := cmd.Run(); err != nil {
		cmd.DumpLog(ctx)
		s.Error("webserver: Failed to add test index.html: ", err)
		return
	}
	cmd = cont.Command(ctx, "python2.7", "-m", "SimpleHTTPServer")
	if err := cmd.Start(); err != nil {
		s.Error("webserver: Failed to run python2: ", err)
		cmd.DumpLog(ctx)
		return
	}
	defer cmd.Wait()
	defer cmd.Kill()

	// Wait for the webserver to actually be up and running.
	testing.ContextLog(ctx, "Waiting for python webserver to start up")
	err := testing.Poll(ctx, func(ctx context.Context) error {
		conn, err := net.DialTimeout("tcp", "penguin.linux.test:8000", time.Second)
		if err != nil {
			return err
		}
		conn.Close()
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second})
	if err != nil {
		s.Error("webserver: Error waiting for python webserver to start up: ", err)
		return
	}
	testing.ContextLog(ctx, "Python webserver startup completed")

	conn, err := cr.NewConn(ctx, "")
	if err != nil {
		s.Error("webserver: Creating renderer failed: ", err)
		return
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

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
