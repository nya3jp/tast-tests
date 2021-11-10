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
		Func: ShillNoErrorLog,
		Desc: "Checks that Shill does not produce any unexpected error logs",
		Contacts: []string{
			"sangchun@google.com",
			"cros-network-health@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
	})
}

// doActions does the actions that we are interested in and
// returns the start time and the end time of them.
func doActions(ctx context.Context, s *testing.State) (time.Time, time.Time) {
	const (
		resetShillTimeout = 30 * time.Second
		shillJob          = "shill"
	)

	// Not part of the actions that we are interested in.
	if err := upstart.StopJob(ctx, shillJob); err != nil {
		s.Fatal("Failed stopping shill: ", err)
	}

	startTime := time.Now()

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

	if wifiAvailable, err := manager.IsAvailable(ctx, shill.TechnologyWifi); err != nil {
		s.Fatal("Error calling IsAvailable: ", err)
	} else if !wifiAvailable {
		s.Fatal("WiFi not available")
	}

	if err := upstart.StopJob(ctx, shillJob); err != nil {
		s.Fatal("Failed stopping shill: ", err)
	}

	endTime := time.Now()

	// Not part of the actions that we are interested in.
	if err := upstart.RestartJob(ctx, shillJob); err != nil {
		s.Fatal("Failed starting shill: ", err)
	}

	return startTime, endTime
}

type subEntry struct {
	Program  string
	FileName string
	Message  string
}

func ShillNoErrorLog(ctx context.Context, s *testing.State) {
	sr, err := syslog.NewReader(ctx, syslog.SourcePath(syslog.NetLogFile))
	if err != nil {
		s.Fatal("Failed to open the net log file: ", err)
	}
	defer sr.Close()

	startTime, endTime := doActions(ctx, s)

	numLogLines := 0
	var entries []*syslog.Entry
	for {
		e, err := sr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			s.Fatal("Failed to read the network log: ", err)
		}
		if e.Timestamp.Before(startTime) ||
			e.Timestamp.After(endTime) {
			continue
		}
		numLogLines++
		if e.Severity != "ERR" {
			continue
		}
		entries = append(entries, e)
	}

	var subEntries []subEntry
	for _, entry := range entries {
		extractSubEntryRegex := regexp.MustCompile("^.*!?(\\[(.*)\\([-]?\\d+\\)\\])(.*)$")
		matched := extractSubEntryRegex.FindStringSubmatch(entry.Content)
		if len(matched) < 4 {
			subEntries = append(subEntries, subEntry{entry.Program, "", entry.Content})
		} else {
			subEntries = append(subEntries, subEntry{entry.Program, matched[2], matched[3]})
		}
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

	s.Log("Number of lines considered: ", numLogLines)
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
