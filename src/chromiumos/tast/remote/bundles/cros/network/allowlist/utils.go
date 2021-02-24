// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package allowlist

import (
	"context"
	"encoding/json"
	"io/ioutil"

	"chromiumos/tast/errors"
	"chromiumos/tast/services/cros/network"
)

type allowlist struct {
	Chromeos  []string
	Extension []string
	Android   []string
}

// SetupFirewall reads the hostnames from `path` and calls the network.Allowlist service to setup a firewall
// which will only allow connections to the specified hosts.
func SetupFirewall(ctx context.Context, path string, arc, ext bool, al *network.AllowlistServiceClient) error {
	j, err := ioutil.ReadFile(path)
	if err != nil {
		return errors.Wrap(err, "failed to read standard config file")
	}

	var a allowlist
	if err := json.Unmarshal([]byte(j), &a); err != nil {
		return errors.Wrap(err, "error decoding json file")
	}
	hosts := a.Chromeos

	if ext || arc {
		hosts = append(hosts, a.Extension...)
	}
	if arc {
		hosts = append(hosts, a.Android...)
	}

	if _, err := (*al).SetupFirewall(ctx, &network.SetupFirewallRequest{
		Hostnames: hosts}); err != nil {
		return err
	}

	return nil
}
