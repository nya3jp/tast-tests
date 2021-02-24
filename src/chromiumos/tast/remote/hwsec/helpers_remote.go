// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

/*
This file implements miscellaneous and unsorted helpers.
*/

import (
	"context"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// CmdHelperRemoteImpl implements the helper functions for CmdHelperRemote
type CmdHelperRemoteImpl struct {
	h *hwsec.CmdTPMClearHelper
	d *dut.DUT
}

// CmdHelperRemote extends the function set of hwsec.CmdHelper
type CmdHelperRemote struct {
	hwsec.CmdTPMClearHelper
	CmdHelperRemoteImpl
}

// AttestationHelperRemote extends the function set of hwsec.AttestationHelper
type AttestationHelperRemote struct {
	hwsec.AttestationHelper
}

// FullHelperRemote extends the function set of hwsec.FullHelper
type FullHelperRemote struct {
	hwsec.FullHelper
	CmdHelperRemoteImpl
}

// NewHelper creates a new hwsec.CmdTPMClearHelper instance that make use of the functions
// implemented by CmdRunnerRemote.
func NewHelper(r hwsec.CmdRunner, d *dut.DUT) (*CmdHelperRemote, error) {
	cmdHelper := hwsec.NewCmdHelper(r)
	tpmClearer := NewTPMClearer(r, cmdHelper.DaemonController(), d)
	tpmClearHelper := hwsec.NewTPMClearHelper(tpmClearer)
	cmdTpmHelper := hwsec.NewCmdTPMClearHelper(cmdHelper, tpmClearHelper)
	return &CmdHelperRemote{*cmdTpmHelper, CmdHelperRemoteImpl{cmdTpmHelper, d}}, nil
}

// NewAttestationHelper creates a new hwsec.AttestationHelper instance that make use of the functions
// implemented by AttestationHelperRemote.
func NewAttestationHelper(d *dut.DUT, h *testing.RPCHint) (*AttestationHelperRemote, error) {
	ac, err := NewAttestationDBus(d, h)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create attestation client")
	}
	helper := hwsec.NewAttestationHelper(ac)
	return &AttestationHelperRemote{*helper}, nil
}

// NewFullHelper creates a new hwsec.FullHelper with a remote AttestationClient.
func NewFullHelper(r hwsec.CmdRunner, d *dut.DUT, h *testing.RPCHint) (*FullHelperRemote, error) {
	ac, err := NewAttestationDBus(d, h)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create attestation client")
	}
	cmdHelper := hwsec.NewCmdHelper(r)
	attestationHelper := hwsec.NewAttestationHelper(ac)
	tpmClearer := NewTPMClearer(r, cmdHelper.DaemonController(), d)
	tpmClearHelper := hwsec.NewTPMClearHelper(tpmClearer)
	cmdTpmHelper := hwsec.NewCmdTPMClearHelper(cmdHelper, tpmClearHelper)
	helper := hwsec.NewFullHelper(cmdTpmHelper, attestationHelper)
	return &FullHelperRemote{*helper, CmdHelperRemoteImpl{&helper.CmdTPMClearHelper, d}}, nil
}

// Reboot reboots the DUT
func (h *CmdHelperRemoteImpl) Reboot(ctx context.Context) error {
	if err := h.d.Reboot(ctx); err != nil {
		return err
	}
	dCtrl := h.h.DaemonController()
	// Waits for all the daemons of interest to be ready because the asynchronous initialization of dbus service could complete "after" the booting process.
	if err := dCtrl.WaitForAllDBusServices(ctx); err != nil {
		return errors.Wrap(err, "failed to wait for hwsec D-Bus services to be ready")
	}
	return nil
}
