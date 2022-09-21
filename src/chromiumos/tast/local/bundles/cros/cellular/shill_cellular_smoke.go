// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/cellular"
	"chromiumos/tast/local/modemmanager"
	"chromiumos/tast/testing"
)

// Note: This test enables and connects to Cellular if not already enabled or connected.

func init() {
	testing.AddTest(&testing.Test{
		Func: ShillCellularSmoke,
		Desc: "Verifies that traffic can be sent over the Cellular network",
		Contacts: []string{
			"stevenjb@google.com",
			"chromeos-cellular-team@google.com",
		},
		Attr:    []string{"group:cellular", "cellular_unstable", "cellular_sim_active"},
		Fixture: "cellular",
		Timeout: 1 * time.Minute,
	})
}

func ShillCellularSmoke(ctx context.Context, s *testing.State) {
	if _, err := modemmanager.NewModemWithSim(ctx); err != nil {
		s.Fatal("Could not find MM dbus object with a valid sim: ", err)
	}

	helper, err := cellular.NewHelper(ctx)
	if err != nil {
		s.Fatal("Failed to create cellular.Helper: ", err)
	}
	// Enable and get service to set autoconnect based on test parameters.
	if _, err := helper.Enable(ctx); err != nil {
		s.Fatal("Failed to enable modem")
	}

	verifyNetworkConnectivity := func(ctx context.Context) error {
		// This URL comes from src/third_party/autotest/files/client/cros/network.py.
		// Code for the app is here: https://chromereviews.googleplex.com/2390012/
		const hostName = "testing-chargen.appspot.com"
		// This pattern also comes from src/third_party/autotest/files/client/cros/network.py
		// and is undocumented.
		const downloadBytes = 65536
		fetchURL := fmt.Sprintf("http://%s/download?size=%d", hostName, downloadBytes)
		s.Log("Fetch URL: ", fetchURL)

		// Get data from |fetchURL| and confirm that the correct number of bytes are received.
		resp, err := http.Get(fetchURL)
		if err != nil {
			return errors.Wrapf(err, "error fetching data from URL %q", fetchURL)
		}
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return errors.Wrap(err, "error reading data")
		}
		bytesRead := len(body)
		if bytesRead != downloadBytes {
			return errors.Errorf("read wrong number of bytes: got %d, want %d", bytesRead, downloadBytes)
		}
		return nil
	}

	if err := helper.RunTestOnCellularInterface(ctx, verifyNetworkConnectivity); err != nil {
		s.Fatal("Failed to run test on cellular interface: ", err)
	}
}
