// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifiutil

import (
	"context"

	"chromiumos/tast/common/network/iw"
	"chromiumos/tast/errors"
	remoteiw "chromiumos/tast/remote/network/iw"
	"chromiumos/tast/ssh"
)

// CheckTDLSSupport verifies that TDLS is supported according to the driver.
func CheckTDLSSupport(ctx context.Context, conn *ssh.Conn) error {
	phys, _, err := remoteiw.NewRemoteRunner(conn).ListPhys(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get capabilities")
	}
	// Basing on two assumptions:
	// 1. All phys of the same modem will have the same capabilities.
	// 2. We support only one WiFi modem per DUT.
	checkCommand := func(phys []*iw.Phy, command string) bool {
		for _, c := range phys[0].Commands {
			if c == command {
				return true
			}
		}
		return false
	}
	if checkCommand(phys, "tdls_oper") && checkCommand(phys, "tdls_mgmt") {
		return nil
	}
	return errors.New("Device does not declare full TDLS support")
}

// Scan facilitates running iw scan.
func Scan(ctx context.Context, conn *ssh.Conn, ifName, ssid string) error {
	_, err := remoteiw.NewRemoteRunner(conn).TimedScan(ctx, ifName, nil, []string{ssid})
	return err
}
