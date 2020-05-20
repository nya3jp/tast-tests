// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"fmt"
	"io/ioutil"
	"path"
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

	// Even if we got a WiFi device, iface, from shill, that does not imply
	// that wpa_supplicant of the WiFi device is up and running. Poll for a
	// successful wpa_cli ping for 5 seconds to avoid false negative.
	wRunner := wpacli.NewRunner(&cmd.LocalCmdRunner{NoLogOnError: true})
	var cmdOut []byte
	dumpCmdOut := func(cmdOut []byte) string {
		filename := "wpa_cli.stdout"
		outDir := s.OutDir()
		outPath := path.Join(outDir, filename)
		err := ioutil.WriteFile(outPath, cmdOut, 0644)
		if err != nil {
			return fmt.Sprintf("failed to write output to %s: %s", outPath, err)
		}
		return path.Join("${tast_result}/tests", path.Base(outDir), filename)
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var err2 error
		cmdOut, err2 = wRunner.Ping(ctx, iface)
		return err2
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		s.Fatalf("Failed to ping wpa_supplicant: %s; command output: %s", err, dumpCmdOut(cmdOut))
	}

}
