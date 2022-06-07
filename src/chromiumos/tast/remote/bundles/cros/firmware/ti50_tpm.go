// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"encoding/json"
	"time"

	"chromiumos/tast/common/firmware/ti50"
	"chromiumos/tast/remote/firmware/ti50/fixture"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:    Ti50Tpm,
		Desc:    "Test TPM functionality of ti50 in remote environment(Andreiboard connected to devboardsvc host)",
		Timeout: 30 * time.Second,
		Contacts: []string{
			"aluo@chromium.org",            // Test Author
			"chromeos-firmware@google.com", // CrOS Firmware Developers
		},
		Attr:    []string{"group:firmware"},
		Fixture: fixture.Ti50,
	})
}

func Ti50Tpm(ctx context.Context, s *testing.State) {
	f := s.FixtValue().(*fixture.Value)

	board, err := f.DevBoard(ctx, 4096, time.Second)
	if err != nil {
		s.Fatal("Could not get board: ", err)
	}

	err = board.Open(ctx)
	if err != nil {
		s.Fatal("Open console port: ", err)
	}
	// Wait a little for opentitantool to take over the console, this will test
	// that flashing still works after the console command.
	testing.Sleep(ctx, 5*time.Second)

	i := ti50.NewCrOSImage(board)
	outStr, err := i.Command(ctx, "version")
	if err != nil {
		s.Fatal("Console version: ", err)
	}

	data, err2 := board.OpenTitanToolCommand(ctx, "tpm", "read-register", "DID_VID")
	if err2 != nil {
		s.Fatal("OpenTitanToolCommand: ", err2)
	}
	var val map[string]interface{}
	json.Unmarshal(data, &val)

	b, err := json.MarshalIndent(val, "", "  ")
	if err != nil {
		s.Fatal("MarshalIndent: ", err)
	}
	if val["hexdata"].(string) != "66664a50" {
		s.Fatal("Unexpected: ", val["hexdata"])
	}
	s.Log("Value: ", val["uint32"])
	s.Log("Output: ", string(b))

	testing.ContextLog(ctx, "Version of Ti50: ")
	testing.ContextLog(ctx, outStr)
}
