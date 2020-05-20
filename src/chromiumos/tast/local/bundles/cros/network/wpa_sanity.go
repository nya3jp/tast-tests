// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/common/network/wpacli"
	"chromiumos/tast/local/network/cmd"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WPASanity,
		Desc:         "Verifies wpa_supplicant is up and running",
		Contacts:     []string{"deanliao@google.com", "chromeos-kernel-wifi@google.com"},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"wifi", "shill-wifi"},
	})
}

func WPASanity(ctx context.Context, s *testing.State) {
	manager, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed creating shill manager proxy: ", err)
	}

	iface, err := shill.WifiInterface(ctx, manager, 5*time.Second)
	if err != nil {
		s.Fatal("Could not get a WiFi interface: ", err)
	}
	s.Log("WiFi interface: ", iface)

	var output string
	logfile := "wpa_cli.log"

	// In case wpa_cli produces multiple line output, in log message we show
	// the path of wpa_cli.log.
	wpacliOutput := func() string {
		if strings.Contains(output, "\n") {
			return "check wpa_cli logfile: /tmp/tast/results/latest/tests/network.WPASanity/" + logfile
		}
		return output
	}

	// Even if we got a WiFi device, iface, from shill, that does not imply
	// that wpa_supplicant of the WiFi device is up and running. Poll for a
	// successful wpa_cli ping for 5 seconds to avoid false negative.
	wpaCli := wpacli.New(&cmd.LocalCmdRunner{})
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		cmdOut, err := wpaCli.Output(ctx, "-i", iface, "ping")
		ioutil.WriteFile(filepath.Join(s.OutDir(), logfile), cmdOut, 0644)
		output = strings.TrimSpace(string(cmdOut))
		return err
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		s.Fatal("Failed to ping wpa_supplicant: " + wpacliOutput())
	}
	if !strings.Contains(output, "PONG") {
		s.Fatal("Failed to see expected PONG reply: " + wpacliOutput())
	}

}
