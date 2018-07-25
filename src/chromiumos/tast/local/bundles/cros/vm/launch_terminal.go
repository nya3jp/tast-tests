// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LaunchTerminal,
		Desc:         "Checks that a container can launch a terminal with the x-terminal-emulator alternative",
		Attr:         []string{"informational"},
		Timeout:      300 * time.Second,
		SoftwareDeps: []string{"chrome_login", "vm_host"},
	})
}

func LaunchTerminal(s *testing.State) {
	const (
		terminalUrlPrefix = "chrome-extension://nkoccljplnhpfnfiajclkommnmllphnl/html/crosh.html?command=vmshell"
	)

	ctx := s.Context()

	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to log in: ", err)
	}
	defer cr.Close(ctx)

	err = vm.SetUpComponent(ctx, vm.StagingComponent)
	if err != nil {
		s.Fatal("Failed to set up component: ", err)
	}

	cont, err := vm.CreateDefaultContainer(ctx, cr.User(), vm.LiveImageServer)
	if err != nil {
		s.Fatal("Failed to set up default container: ", err)
	}
	defer vm.StopConcierge(ctx)

	cmd := cont.Command(ctx, "sudo", "apt-get", "update")
	s.Log("Running ", cmd.Args()...)
	if err = cmd.Run(); err != nil {
		cmd.DumpLog(ctx)
		s.Fatal("Failed to do apt-get update: ", err)
	}

	cmd = cont.Command(ctx, "sudo", "apt-get", "install", "-y", "cros-garcon")
	s.Log("Running ", cmd.Args()...)
	if err = cmd.Run(); err != nil {
		cmd.DumpLog(ctx)
		s.Fatal("Failed to do apt-get dist-upgrade: ", err)
	}

	checkLaunch := func(urlSuffix string, command ...string) {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		cmd = cont.Command(ctx, command...)
		s.Log("Running ", cmd.Args()...)
		if err = cmd.Run(); err != nil {
			cmd.DumpLog(ctx)
			s.Fatal("Failed to launch terminal command in container: ", err)
		}

		s.Log("Waiting for renderer with URL prefix %q and suffix %q", terminalUrlPrefix, urlSuffix)
		conn, err := cr.NewConnForTarget(ctx, func(t *chrome.Target) bool {
			return strings.HasPrefix(t.URL, terminalUrlPrefix) &&
				strings.HasSuffix(t.URL, urlSuffix)
		})
		if err != nil {
			s.Error(err)
		} else {
			conn.Close()
		}
	}

	checkLaunch("", "x-terminal-emulator")

	// When we pass an argument to the x-terminal-emulator alternative, it should
	// then append that as URL parameters which will cause the terminal to
	// execute that command initially.
	checkLaunch("&args[]=--&args[]=vim", "x-terminal-emulator", "vim")
}
