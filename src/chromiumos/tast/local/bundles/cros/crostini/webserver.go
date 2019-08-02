// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"fmt"
	"net"
	"strconv"
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

	sockaddrs := []struct {
		Addr net.IP
		Port uint16
	}{
		{net.IPv4(127, 0, 0, 1), 6789}, // An unprivileged port listening on localhost should be tunneled.
		{net.IPv4zero, 999},            // Privileged ports will be accessible via penguin.linux.test but not localhost.
		{net.IPv4zero, 8000},           // Common dev webserver port.
		{net.IPv4zero, 12345},          // Uncommon dev webserver port.
	}

	for _, sockaddr := range sockaddrs {
		cmd = cont.Command(ctx, "sudo", "python3", "-m", "http.server",
			strconv.FormatUint(uint64(sockaddr.Port), 10),
			"--bind", sockaddr.Addr.String())
		if err := cmd.Start(); err != nil {
			s.Error("webserver: Failed to run python3: ", err)
			cmd.DumpLog(ctx)
			return
		}
		defer cmd.Wait()
		defer cmd.Kill()
	}

	// Wait for the webserver to actually be up and running, and for chunnel
	// to be ready to accept connections.
	testing.ContextLog(ctx, "Waiting for python webserver to start up")
	lastPort := sockaddrs[len(sockaddrs)-1].Port
	err := testing.Poll(ctx, func(ctx context.Context) error {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", lastPort), time.Second)
		if err != nil {
			return err
		}
		conn.Close()
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second})
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

	for _, sockaddr := range sockaddrs {
		checkUrls := []string{}

		if sockaddr.Addr.IsLoopback() || sockaddr.Addr.IsUnspecified() {
			// Localhost tunneling only works on unprivileged ports.
			if sockaddr.Port > 1023 {
				checkUrls = append(checkUrls, fmt.Sprintf("http://127.0.0.1:%d", sockaddr.Port))
				checkUrls = append(checkUrls, fmt.Sprintf("http://[::1]:%d", sockaddr.Port))
			}
		}

		if sockaddr.Addr.IsUnspecified() {
			checkUrls = append(checkUrls, fmt.Sprintf("http://penguin.linux.test:%d", sockaddr.Port))
		}

		for _, url := range checkUrls {
			checkNavigation(url)
		}
	}
}
