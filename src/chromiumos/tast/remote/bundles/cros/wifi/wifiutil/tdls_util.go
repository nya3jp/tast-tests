// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifiutil

import (
	"context"
	"regexp"
	"strconv"

	"chromiumos/tast/common/network/iw"
	"chromiumos/tast/errors"
	remoteiw "chromiumos/tast/remote/network/iw"
	"chromiumos/tast/ssh"
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
		return false, errors.Wrap(err, "failed to call command")
	}
	return ExpectOutput(string(ret), lookup), nil
}

// CheckTDLSSupport verifies that TDLS is supported according to the driver.
func CheckTDLSSupport(ctx context.Context, conn *ssh.Conn) error {
	phys, _, err := remoteiw.NewRemoteRunner(conn).ListPhys(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get capabilities")
	}
	checkCommand := func(phys []*iw.Phy, command string) bool {
		for _, p := range phys {
			for _, c := range p.Commands {
				if c == command {
					return true
				}
			}
		}
		return false
	}
	if checkCommand(phys, "tdls_oper") && checkCommand(phys, "tdls_oper") {
		return nil
	}
	return errors.New("Device does not declare full TDLS support")
}
