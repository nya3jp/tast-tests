// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hps

import (
	"context"
	"os"
	"strconv"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/hps/hpsutil"
	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/bundles/cros/hps/utils"
	"chromiumos/tast/rpc"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CameraboxPresence,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify that the HPS can correctly detect presence from another tablet",
		Data:         []string{hpsutil.PersonPresentPageArchiveFilename},
		Contacts: []string{
			"eunicesun@google.com",
			"mblsha@google.com",
			"chromeos-hps-swe@google.com",
		},
		Attr:         []string{"group:camerabox", "group:hps"},
		Timeout:      20 * time.Minute,
		SoftwareDeps: []string{"hps", "chrome", caps.BuiltinCamera},
		Vars:         []string{"tablet"},
	})
}

func CameraboxPresence(ctx context.Context, s *testing.State) {
	dut := s.DUT()

	// Creating hps context.
	hctx, err := hpsutil.NewHpsContext(ctx, "", hpsutil.DeviceTypeBuiltin, s.OutDir(), dut.Conn())
	if err != nil {
		s.Fatal("Error creating HpsContext: ", err)
	}

	// Connecting to the other tablet that will render the picture.
	hostPaths, displayChart, err := utils.SetupDisplay(ctx, s)
	if err != nil {
		s.Fatal("Error setting up display: ", err)
	}

	// Connecting to Taeko.
	cleanupCtx, cancel := ctxutil.Shorten(ctx, time.Minute)
	defer cancel()
	cl, err := rpc.Dial(ctx, dut, s.RPCHint())
	if err != nil {
		s.Fatal("Failed to setup DUT: ", err)
	}
	defer cl.Close(cleanupCtx)

	bindKernelDriver(ctx, dut.Conn(), false)

	for i := 0; i < 2; i++ {
		// for no person
		displayChart.Display(ctx, hostPaths[utils.ZeroPresence])
		if err := numPersonDetect(hctx, strconv.Itoa(i)); err != nil {
			s.Fatal("Failed to run N presence ops: ", err)
		}

		// for one/two person present
		var presenceNum string
		if presenceNum = utils.OnePresence; i != 0 {
			presenceNum = utils.TwoPresence
		}
		displayChart.Display(ctx, hostPaths[presenceNum])
		if err := numPersonDetect(hctx, strconv.Itoa(i)); err != nil {
			s.Fatal("Failed to run N presence ops: ", err)
		}
	}
	bindKernelDriver(ctx, dut.Conn(), true)
}

func numPersonDetect(hctx *hpsutil.HpsContext, feature string) error {
	if _, err := hpsutil.EnablePresence(hctx, feature); err != nil {
		return errors.Wrap(err, "enablePresence failed")
	}

	if _, err := hpsutil.WaitForNPresenceOps(hctx, hpsutil.WaitNOpsBeforeStart, feature); err != nil {
		return errors.Wrap(err, "failed to run N presence ops")
	}

	_, err := hpsutil.WaitForNPresenceOps(hctx, hpsutil.GetNOpsToVerifyPresenceWorks, feature)
	if err != nil {
		return errors.Wrap(err, "failed to run N presence ops")
	}
	return nil
}

// bindKernelDriver performs binding/unbinding the driver based on the boolean given
func bindKernelDriver(ctx context.Context, dconn *ssh.Conn, req bool) (*empty.Empty, error) {
	var err error
	var binded bool
	_, err = os.Stat("/dev/cros-hps")
	if errors.Is(err, os.ErrNotExist) {
		binded = false
	} else if err == nil {
		binded = true
	}

	if req && !binded {
		err = dconn.CommandContext(ctx, "hps", "bind").Run()
	} else if !req && binded {
		err = dconn.CommandContext(ctx, "hps", "unbind").Run()
	}

	if err != nil {
		return nil, errors.Wrap(err, "Unable to bind/unbind kernel driver")
	}

	return &empty.Empty{}, nil
}
