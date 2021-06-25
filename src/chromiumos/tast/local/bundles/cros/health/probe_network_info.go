// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package health

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/croshealthd"
	"chromiumos/tast/testing"
)

type networkInfo struct {
	PortalState    string  `json:"portal_state"`
	State          string  `json:"state"`
	Type           string  `json:"type"`
	GUID           *string `json:"guid"`
	Ipv4Address    *string `json:"ipv4_address"`
	Ipv6Addresses  *string `json:"ipv6_addresses"`
	MacAddress     *string `json:"mac_address"`
	Name           *string `json:"name"`
	SignalStrength *string `json:"signal_strength"`
}

type networkResult struct {
	Networks []networkInfo `json:"networks"`
}

func init() {
	testing.AddTest(&testing.Test{
		Func: ProbeNetworkInfo,
		Desc: "Check that we can probe cros_healthd for network info",
		Contacts: []string{
			"tbegin@google.com",
			"pmoy@google.com",
			"khegde@google.com",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", "diagnostics"},
		Fixture:      "crosHealthdRunning",
	})
}

func ProbeNetworkInfo(ctx context.Context, s *testing.State) {
	params := croshealthd.TelemParams{Category: croshealthd.TelemCategoryNetwork}

	// Helper function to write the result from telem to a file.
	f, err := os.OpenFile(filepath.Join(s.OutDir(), "network_health_telem.txt"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		s.Fatal("Unable to open network_health_telem.txt file: ", err)
	}
	defer f.Close()
	appendResultToFile := func(b []byte) {
		if _, err := f.Write(b); err != nil {
			s.Fatal("Failed to append to network_health_telem.txt file: ", err)
		}
	}

	// If this test is run right after chrome is started, it's possible that the
	// network health information has not been populated. Poll the routine until
	// network information is present.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var err error
		rawData, err := croshealthd.RunTelem(ctx, params, s.OutDir())
		if err != nil {
			s.Fatal("Failed to run telem command: ", err)
		}
		appendResultToFile(rawData)

		dec := json.NewDecoder(strings.NewReader(string(rawData)))
		dec.DisallowUnknownFields()

		var result networkResult
		if err := dec.Decode(&result); err != nil {
			s.Fatalf("Failed to decode network data [%q], err [%v]", rawData, err)
		}

		// Every system should have at least one network device populated. If
		// not, re-poll the routine.
		if len(result.Networks) < 1 {
			return errors.New("no network info populated")
		}

		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		s.Fatal("Timed out waiting for network health info: ", err)
	}
}
