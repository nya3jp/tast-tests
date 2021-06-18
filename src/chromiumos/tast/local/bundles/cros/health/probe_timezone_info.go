// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package health

import (
	"context"
	"encoding/json"
	"strings"

	"chromiumos/tast/local/croshealthd"
	"chromiumos/tast/testing"
)

type timezoneInfo struct {
	Posix  string `json:"posix"`
	Region string `json:"region"`
}

func init() {
	testing.AddTest(&testing.Test{
		Func: ProbeTimezoneInfo,
		Desc: "Check that we can probe cros_healthd for timezone info",
		Contacts: []string{
			"pmoy@google.com",
			"cros-tdm@google.com",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", "diagnostics"},
		Fixture:      "crosHealthdRunning",
	})
}

func ProbeTimezoneInfo(ctx context.Context, s *testing.State) {
	params := croshealthd.TelemParams{Category: croshealthd.TelemCategoryTimezone}
	rawData, err := croshealthd.RunTelem(ctx, params, s.OutDir())
	if err != nil {
		s.Fatal("Failed to get timezone telemetry info: ", err)
	}

	dec := json.NewDecoder(strings.NewReader(string(rawData)))
	dec.DisallowUnknownFields()

	var timezone timezoneInfo
	if err := dec.Decode(&timezone); err != nil {
		s.Fatalf("Failed to decode timezone data %q: %v", rawData, err)
	}

	if timezone.Posix == "" {
		s.Error("Missing posix info")
	}

	if timezone.Region == "" {
		s.Error("Missing region info")
	}
}
