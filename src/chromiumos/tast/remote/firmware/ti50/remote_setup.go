// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ti50

import (
	"context"
	"time"

	"chromiumos/tast/common/firmware/ti50"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/rpc"
	"chromiumos/tast/testing"
)

const (
	// WorkstationMode indicates that the DevBoard under test is connected to the host.
	WorkstationMode string = "workstation"
	// LabMode indicates that the DevBoard under test is connected to a CrOS box in the lab.
	LabMode string = "lab"
)

// ParseTi50TestMode gets the mode or uses the default.
func ParseTi50TestMode(ctx context.Context, mode string) string {
	if mode != LabMode && mode != WorkstationMode {
		testing.ContextLogf(ctx, "\"-var=mode=%s\" not valid, defaulting to %q", mode, WorkstationMode)
		mode = WorkstationMode
	}
	return mode
}

// ParseTi50TestSpiflash gets the spiflash or uses the default.
func ParseTi50TestSpiflash(ctx context.Context, spiflash string) string {
	if spiflash == "" {
		spiflash = "/mnt/host/source/src/platform/cr50-utils/software/tools/SPI/spiflash"
		testing.ContextLog(ctx, "-var=spiflash= not set, defaulting to ", spiflash)
	}
	return spiflash
}

// GetTi50TestBoard gets a DevBoard for testing in either lab or workstation modes.
// TODO(b/197998755): Move into a precondition.
func GetTi50TestBoard(ctx context.Context, dut *dut.DUT, rpcHint *testing.RPCHint, mode, spiflash string, bufLen int, readTimeout time.Duration) (ti50.DevBoard, *rpc.Client, error) {
	mode = ParseTi50TestMode(ctx, mode)
	testing.ContextLogf(ctx, "Using %q mode for Ti50Test", mode)
	spiflash = ParseTi50TestSpiflash(ctx, spiflash)
	testing.ContextLogf(ctx, "Using spiflash at %q for Ti50Test", spiflash)

	var targets []string
	var err error
	if mode == WorkstationMode {
		targets, err = ti50.ListConnectedUltraDebugTargets(ctx)
	} else if mode == LabMode {
		targets, err = ListRemoteUltraDebugTargets(ctx, dut)
	}
	if len(targets) == 0 {
		return nil, nil, errors.Wrap(err, "could not find any UD targets")
	}
	testing.ContextLogf(ctx, "UD Targets: %v, choosing first one found", targets)
	tty := string(targets[0])

	var board ti50.DevBoard
	var rpcClient *rpc.Client

	if mode == WorkstationMode {
		board = ti50.NewConnectedAndreiboard(tty, bufLen, spiflash, readTimeout)
	} else if mode == LabMode {
		rpcClient, err = rpc.Dial(ctx, dut, rpcHint, "cros")
		if err != nil {
			return nil, nil, errors.Wrap(err, "dialing rpc")
		}

		board = NewRemoteAndreiboard(dut, rpcClient.Conn, tty, bufLen, spiflash, readTimeout)
	}
	return board, rpcClient, nil
}
