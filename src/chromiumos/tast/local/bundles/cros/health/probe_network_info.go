// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package health

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/croshealthd"
	"chromiumos/tast/testing"
)

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
		b, err := croshealthd.RunTelem(ctx, params, s.OutDir())
		if err != nil {
			s.Fatal("Failed to run telem command: ", err)
		}
		appendResultToFile(b)

		lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
		// Every response should have the field headers.
		if len(lines) < 1 {
			s.Fatal("Could not find network info header")
		}

		// Verify the header keys are correct.
		header := []string{
			"type", "state", "portal_state", "guid", "name", "signal_strength",
			"mac_address", "ipv4_address", "ipv6_addresses"}
		got := strings.Split(lines[0], ",")
		if !reflect.DeepEqual(got, header) {
			s.Fatalf("Incorrect NetworkInfo keys: got %v; want %v", got, header)
		}

		// Every system should have at least one network device populated. If
		// not, re-poll the routine.
		if len(lines) < 2 {
			return errors.New("no network info populated")
		}

		// Verify that all network devices have the correct number of fields.
		for _, line := range lines[1:] {
			vals := strings.Split(line, ",")
			if len(vals) != len(header) {
				s.Fatalf("Unexpected number of fields in network structure: got: %v, want: %v, fields: %v",
					len(vals), len(header), vals)
			}
		}

		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		s.Fatal("Timed out waiting for network health info: ", err)
	}
}
