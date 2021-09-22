// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package factory

import (
	"bytes"
	"context"
	"encoding/json"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

const (
	testPageTitle = "CrOS Factory"
	testPageType  = "page"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     Kiosk,
		Desc:     "Test if factory UI is running",
		Contacts: []string{"lschyi@google.com", "chromeos-factory-eng@google.com"},
		Attr:     []string{"group:mainline", "informational"},
		Timeout:  time.Minute,
		Fixture:  "ensureToolkit",
		// Skip "nyan_kitty" due to slow reboot speed.
		HardwareDeps: hwdep.D(hwdep.SkipOnModel("kitty")),
	})
}

func Kiosk(ctx context.Context, s *testing.State) {
	// Wait factory test UI show up
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		debugResponse, err := getDebugResponse(ctx, s.DUT().Conn())
		if err != nil {
			return errors.Wrap(err, "failed to connect to debugging port")
		}
		debugEntries, err := getDebugEntries(ctx, debugResponse)
		if err != nil {
			return testing.PollBreak(err)
		}
		if !containsFactoryEntryResponse(debugEntries) {
			return errors.New("factory test UI page not ready")
		}
		return nil
	}, &testing.PollOptions{Interval: time.Second}); err != nil {
		s.Fatal("Device not showing factory test UI: ", err)
	}
}

func getDebugResponse(ctx context.Context, conn *ssh.Conn) ([]byte, error) {
	probeDebugPortCmd := conn.CommandContext(ctx, "curl", "localhost:9222/json/list")
	return probeDebugPortCmd.Output()
}

func getDebugEntries(ctx context.Context, debugResponse []byte) ([]*debugEntry, error) {
	responseReader := bytes.NewReader(debugResponse)
	decoder := json.NewDecoder(responseReader)
	var debugEntries []*debugEntry
	if err := decoder.Decode(&debugEntries); err != nil {
		return nil, errors.Wrap(err, "failed to parse probe response")
	}
	return debugEntries, nil
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
	Type  string `json:"type"`
}
