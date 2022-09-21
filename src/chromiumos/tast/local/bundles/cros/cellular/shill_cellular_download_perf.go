// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
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
		Func: ShillCellularDownloadPerf,
		Desc: "Verifies that large files can be downloaded over the network and records the average speed",
		Contacts: []string{
			"ejcaruso@google.com",
			"chromeos-cellular-team@google.com",
		},
		Attr:    []string{"group:cellular", "cellular_unstable", "cellular_sim_active"},
		Fixture: "cellular",
		Timeout: 10 * time.Minute,
	})
}

func ShillCellularDownloadPerf(ctx context.Context, s *testing.State) {
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

	type subtest struct {
		name string
		url  string
		size int
	}

	ipv4Subtests := []subtest{
		{"ipv4-5mb", "http://ipv4.download.thinkbroadband.com/5MB.zip", 5242880},
		{"ipv4-20mb", "http://ipv4.download.thinkbroadband.com/20MB.zip", 20971520},
		{"ipv4-100mb", "http://ipv4.download.thinkbroadband.com/100MB.zip", 104857600},
	}
	ipv6Subtests := []subtest{
		{"ipv6-5mb", "http://ipv6.download.thinkbroadband.com/5MB.zip", 5242880},
		{"ipv6-20mb", "http://ipv6.download.thinkbroadband.com/20MB.zip", 20971520},
		{"ipv6-100mb", "http://ipv6.download.thinkbroadband.com/100MB.zip", 104857600},
	}

	ipv4, ipv6, err := helper.GetNetworkProvisionedCellularIPTypes(ctx)
	if err != nil {
		s.Fatal("Failed to get provisioned IP families")
	}

	subtestsToRun := make([]subtest, 0, len(ipv4Subtests)+len(ipv6Subtests))
	if ipv4 {
		for _, st := range ipv4Subtests {
			subtestsToRun = append(subtestsToRun, st)
		}
	}
	if ipv6 {
		for _, st := range ipv6Subtests {
			subtestsToRun = append(subtestsToRun, st)
		}
	}

	if len(subtestsToRun) == 0 {
		s.Fatal("No IP networks were provisioned")
	}

	downloadFile := func(ctx context.Context, st subtest) error {
		// Get data from |st.url| and confirm that the correct number of bytes are received.
		resp, err := http.Get(st.url)
		if err != nil {
			return errors.Wrapf(err, "error fetching data from URL %q", st.url)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			return errors.Errorf("HTTP GET returned bad status code: got %d, want 200", resp.StatusCode)
		}

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return errors.Wrap(err, "error reading data")
		}
		bytesRead := len(body)
		if bytesRead != st.size {
			return errors.Errorf("read wrong number of bytes: got %d, want %d", bytesRead, st.size)
		}
		return nil
	}

	for _, st := range subtestsToRun {
		s.Run(ctx, st.name, func(ctx context.Context, s *testing.State) {
			downloadFunc := func(ctx context.Context) error {
				return downloadFile(ctx, st)
			}
			if err := helper.RunTestOnCellularInterface(ctx, downloadFunc); err != nil {
				s.Error("Failed to download file: ", err)
			}
		})
	}
}
