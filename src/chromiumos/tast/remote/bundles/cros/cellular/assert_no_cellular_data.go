// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"time"

	"chromiumos/tast/remote/cellular/callbox/manager"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: AssertNoCellularData,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc: "Asserts that cellular data connection can be disrupted after establised. The test establishes a connection to the appropriate CMW500 callbox. Then it reduces the cellular signal to undetectable levels and asserts that there is no data connection anymore.",
		Contacts: []string{
			"latware@google.com",
			"chromeos-cellular-team@google.com",
		},
		Attr:         []string{},
		ServiceDeps:  []string{"tast.cros.example.ChromeService"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "callboxManagedFixture",
		Timeout:      5 * time.Minute,
	})
}

func AssertNoCellularData(ctx context.Context, s *testing.State) {
	testURL := "google.com"
	cellularInterface := "rmnet_data0"
	dutConn := s.DUT().Conn()
	tf := s.FixtValue().(*manager.TestFixture)
	tf.ConnectToCallbox(ctx, s, dutConn, &manager.ConfigureCallboxRequestBody{
		Hardware:     "CMW",
		CellularType: "LTE",
		ParameterList: []string{
			"band", "2",
			"bw", "20",
			"mimo", "2x2",
			"tm", "1",
			"pul", "0",
			"pdl", "high",
		},
	}, cellularInterface)

	// break cellular connection
        tf.ConnectToCallbox(ctx, s, dutConn, &manager.ConfigureCallboxRequestBody{
                Hardware:     "CMW",
                CellularType: "LTE",
                ParameterList: []string{
                        "band", "2",
                        "bw", "20",
                        "mimo", "2x2",
                        "tm", "1",
                        "pul", "0",
                        "pdl", "disconnected",
                },
        }, cellularInterface)

	_, err := dutConn.CommandContext(ctx, "curl", "--interface", cellularInterface, testURL).Output()
	if err == nil {
		s.Fatalf("Curled %q on DUT using cellular interface and did not get an error: %v", testURL, err)
	}
}
