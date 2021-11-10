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
// retruns the start time and the end time of them.
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

type allowedSubEntry struct {
	Program      string
	FileName     string
	MessageRegex string
	Counter      int
}

func ShillNoErrorLog(ctx context.Context, s *testing.State) {
	const (
		netLogFile = "/var/log/net.log"
	)

	var allowedSubEntries = []allowedSubEntry{
		{"patchpaneld", "ethod_invoker.h", ".*CallMethodAndBlockWithTimeout.*", 0},
		{"patchpaneld", "manager.cc", ".*Invalid namespace name.*", 0},
		{"patchpaneld", "object_proxy.cc", ".*Failed to call method.*", 0},
		{"patchpaneld", "scoped_ns.cc", ".*Could not open namespace.*", 0},
		{"patchpaneld", "shill_client.cc", ".*Unable to get manager properties.*", 0},
		{"shill", "cellular_capability_3gpp.cc", ".*No slot found for.*", 0},
		{"shill", "cellular.cc", ".*StartModem failed.*", 0},
		{"shill", "device_info.cc", ".*Add Link message for.*does not have .*", 0},
		{"shill", "dns_client.cc", ".*No valid DNS server addresses.*", 0},
		{"shill", "ethod_invoker.h", ".*CallMethodAndBlockWithTimeout.*", 0},
		{"shill", "http_request.cc", ".*Failed to start DNS client.*", 0},
		{"shill", "object_proxy.cc", ".*Failed to call method.*", 0},
		{"shill", "portal_detector.cc", ".*HTTP probe failed to start.*", 0},
		{"shill", "unknown", ".*", 0},
		{"shill", "utils.cc", ".*AddDBusError.*", 0},
		{"shill", "wifi.cc", ".*does not support MAC address randomization.*", 0},
		{"wpa_supplicant", "", ".*Permission denied.*", 0},
	}

	sr, err := syslog.NewReader(ctx, syslog.SourcePath(netLogFile))
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

	var unexpected []subEntry
	for _, e := range subEntries {
		allowed := false
		for i, r := range allowedSubEntries {
			if r.Program != e.Program || r.FileName != e.FileName {
				continue
			}
			if matched, _ := regexp.MatchString(r.MessageRegex, e.Message); matched {
				allowedSubEntries[i].Counter++
				allowed = true
				break
			}
		}
		if !allowed {
			unexpected = append(unexpected, e)
		}
	}

	s.Log("The number of lines considered: ", numLogLines)
	for _, r := range allowedSubEntries {
		s.Log(r.Counter, " * (", r.Program, ", ", r.FileName, ", ", r.MessageRegex, ")")
	}

	if len(unexpected) != 0 {
		for _, e := range unexpected {
			s.Log(e)
		}
		s.Fatal("The number of unexpected error lines: ", len(unexpected))
	}
}
