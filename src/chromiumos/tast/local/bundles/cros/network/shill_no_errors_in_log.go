// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"io"
	"regexp"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ShillNoErrorsInLog,
		Desc: "Checks that Shill does not produce any unexpected error logs",
		Contacts: []string{
			"sangchun@google.com",
			"cros-network-health@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
	})
}

const shillJob = "shill"

// startShillAndWaitForNetworks does the actions that we are interested in.
// Shill should be stopped before calling this function and should be restarted
// after calling this function.
func startShillAndWaitForNetworks(ctx context.Context, s *testing.State) {
	const resetShillTimeout = 30 * time.Second

	if err := upstart.RestartJob(ctx, shillJob); err != nil {
		s.Fatal("Failed starting shill: ", err)
	}

	manager, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed creating shill manager proxy: ", err)
	}

	// Wait until a service is connected.
	expectProps := map[string]interface{}{
		shillconst.ServicePropertyIsConnected: true,
	}
	if _, err := manager.WaitForServiceProperties(ctx, expectProps, resetShillTimeout); err != nil {
		s.Fatal("Failed to wait for connected service: ", err)
	}

	if ethernetAvailable, err := manager.IsAvailable(ctx, shill.TechnologyEthernet); err != nil {
		s.Fatal("Error calling IsAvailable: ", err)
	} else if !ethernetAvailable {
		s.Fatal("Ethernet not available")
	}

	// Wait for WiFi to come up if it is available.
	if wifiAvailable, err := manager.IsAvailable(ctx, shill.TechnologyWifi); err != nil {
		s.Fatal("Error calling IsAvailable: ", err)
	} else if !wifiAvailable {
		s.Log("WiFi not available")
	}

	if err := upstart.StopJob(ctx, shillJob); err != nil {
		s.Fatal("Failed stopping shill: ", err)
	}
}

func ShillNoErrorsInLog(ctx context.Context, s *testing.State) {
	if err := upstart.StopJob(ctx, shillJob); err != nil {
		s.Fatal("Failed stopping shill: ", err)
	}
	defer upstart.RestartJob(ctx, shillJob)

	sr, err := syslog.NewReader(ctx, syslog.SourcePath(syslog.NetLogFile), syslog.Severities(syslog.Err))
	if err != nil {
		s.Fatal("Failed to open the net log file: ", err)
	}
	defer sr.Close()
	startShillAndWaitForNetworks(ctx, s)
	endTime := time.Now()

	type subEntry struct {
		Program  string
		FileName string
		Message  string
	}
	var subEntries []subEntry

	for {
		e, err := sr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			s.Fatal("Failed to read the network log: ", err)
		}
		if e.Timestamp.After(endTime) {
			break
		}
		subEntries = append(subEntries, subEntry{e.Program, syslog.ExtractFileName(*e), e.Content})
	}

	allowedEntries := shillconst.InitializeAllowedEntries()
	var unexpected []subEntry
	for _, e := range subEntries {
		allowed := false
		for i, r := range allowedEntries {
			if r.Program != e.Program || r.FileName != e.FileName {
				continue
			}
			if matched, _ := regexp.MatchString(r.MessageRegex, e.Message); matched {
				allowedEntries[i].Counter++
				allowed = true
				break
			}
		}
		if !allowed {
			unexpected = append(unexpected, e)
		}
	}

	for _, r := range allowedEntries {
		s.Log(r.Counter, " * ", r.Program, ":", r.FileName, ":", r.MessageRegex)
	}

	if len(unexpected) != 0 {
		s.Log("Unexpected errors: ")
		for _, e := range unexpected {
			s.Log(e)
		}
		s.Fatal("Number of unexpected error lines: ", len(unexpected))
	}
}
