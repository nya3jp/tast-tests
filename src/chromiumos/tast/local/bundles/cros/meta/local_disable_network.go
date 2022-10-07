// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"
	"strconv"
	"time"

	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

var (
	sleepDuration = testing.RegisterVarString(
		"meta.LocalDisableNetwork.SleepDuration",
		"180",
		"The duration of the sleep in seconds")
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LocalDisableNetwork,
		Desc:         "Disable network temporary and then reenable it",
		Contacts:     []string{"tast-owners@google.com", "seewaifu@google.com"},
		BugComponent: "b:1034625",
		Timeout:      time.Minute * 4,
	})
}

// LocalDisableNetwork will disable the network, sleep for a number of seconds
// according the runtime variable meta.LocalDisableNetwork.SleepDuration, and
// reenable the network at the end.
func LocalDisableNetwork(ctx context.Context, s *testing.State) {
	manager, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed creating shill manager proxy: ", err)
	}

	ethEnabled, err := manager.IsEnabled(ctx, shill.TechnologyEthernet)
	if err != nil {
		s.Fatal("Failed to check if ethernet is enabled: ", err)
	}
	if ethEnabled {
		ethEnableFunc, err := manager.DisableTechnologyForTesting(ctx, shill.TechnologyEthernet)
		if err != nil {
			s.Fatal("Failed to disable ethernet: ", err)
		}
		s.Log("Ethernet has been disabled")
		defer func() {
			ethEnableFunc(ctx)
			s.Log("Ethernet has been re-enabled")
		}()
	}

	wifiEnabled, err := manager.IsEnabled(ctx, shill.TechnologyWifi)
	if err != nil {
		s.Fatal("Failed to check if wifi is enabled: ", err)
	}
	if wifiEnabled {
		wifiEnableFunc, err := manager.DisableTechnologyForTesting(ctx, shill.TechnologyWifi)
		if err != nil {
			s.Fatal("Failed to disable wifi: ", err)
		}
		s.Log("Wifi has been disabled")
		defer func() {
			wifiEnableFunc(ctx)
			s.Log("Wifi has been re-enabled")
		}()
	}

	sec, err := strconv.Atoi(sleepDuration.Value())
	if err != nil {
		s.Fatalf("Bad value %v is set for variable %v: %v", sleepDuration.Value(), sleepDuration.Name(), err)
	}
	s.Logf("Sleeping for %d seconds", sec)
	testing.Sleep(ctx, time.Duration(sec)*time.Second)
}
