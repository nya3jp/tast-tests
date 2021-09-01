// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package factory

import (
	"bytes"
	"context"
	"encoding/json"
	"time"

	"chromiumos/tast/remote/bundles/cros/factory/pre"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

const (
	testPageTitle = "CrOS Factory"
	testPageType  = "page"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     Browser,
		Desc:     "Test if factory UI is running",
		Contacts: []string{"lschyi@google.com", "chromeos-factory-eng@google.com"},
		Attr:     []string{"group:mainline", "informational"},
		Timeout:  time.Minute,
		Pre:      pre.GetToolkitEnsurer(),
		// Skip "nyan_kitty" due to slow reboot speed.
		HardwareDeps: hwdep.D(hwdep.SkipOnModel("kitty")),
	})
}

func Browser(ctx context.Context, s *testing.State) {
	conn := s.DUT().Conn()
	probeDebugPortCmd := conn.CommandContext(ctx, "curl", "localhost:9222/json/list")
	probeResponse, err := probeDebugPortCmd.Output()
	if err != nil {
		s.Fatal("Failed to connect to debugging port: ", err)
	}

	responseReader := bytes.NewReader(probeResponse)
	decoder := json.NewDecoder(responseReader)
	debugEntries := make([]*debugEntry, 0)
	if decoder.Decode(&debugEntries); err != nil {
		s.Fatal("Failed to parse probe response: ", err)
	}
	if !containsFactoryEntryResponse(debugEntries) {
		s.Fatalf("%s is not running", testPageTitle)
	}
}

func containsFactoryEntryResponse(entries []*debugEntry) bool {
	for _, e := range entries {
		if e.Title == testPageTitle && e.Type == testPageType {
			return true
		}
	}
	return false
}

type debugEntry struct {
	Title string `json:"title"`
	Type  string `type:"type"`
}
