// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifiutil

import (
	"context"
	"net"
	"regexp"
	"strconv"

	"chromiumos/tast/errors"
	"chromiumos/tast/remote/network/ip"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing/hwdep"
)

// UniqueAPName returns AP name to be used in packet dumps.
func UniqueAPName() string {
	id := strconv.Itoa(apID)
	apID++
	return id
}

// ExpectOutput checks if string contains matching regexp.
func ExpectOutput(str, lookup string) bool {
	re := regexp.MustCompile(lookup)
	return re.MatchString(str)
}

// RunAndCheckOutput runs command and checks if the output matches expected regexp.
func RunAndCheckOutput(ctx context.Context, cmd *ssh.Cmd, lookup string) (bool, error) {
	ret, err := cmd.Output()
	if err != nil {
		return false, errors.Wrap(err, "failed to call command, err")
	}
	return ExpectOutput(string(ret), lookup), nil
}

// CheckTDLSSupport verifies that TDLS is supported according to the driver.
func CheckTDLSSupport(ctx context.Context, conn *ssh.Conn) error {
	ret, err := conn.CommandContext(ctx, "iw", "phy").Output()
	if err != nil {
		return errors.Wrap(err, "failed to call command, err")
	}
	if !ExpectOutput(string(ret), "tdls_oper") || !ExpectOutput(string(ret), "tdls_mgmt") {
		return errors.New("device does not declare TDLS support")
	}
	return nil
}

// GetMAC returns MAC address of the requested interface on the device accessible through SSH connection.
func GetMAC(ctx context.Context, conn *ssh.Conn, ifName string) (net.HardwareAddr, error) {
	ipr := ip.NewRemoteRunner(conn)
	hwMAC, err := ipr.MAC(ctx, ifName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get MAC of WiFi interface")
	}
	return hwMAC, nil
}

// TDLSHwDeps returns dependencies for the test.
func TDLSHwDeps() hwdep.Condition {
	return hwdep.SkipOnWifiDevice(
		// mwifiex in 3.10 kernel does not support it.
		hwdep.Marvell88w8897SDIO, hwdep.Marvell88w8997PCIE)
}
