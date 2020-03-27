// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

/*
This file implements functions to check or switch the DUT's boot mode.
*/

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"

	fwCommon "chromiumos/tast/common/firmware"
	"chromiumos/tast/errors"
	fwpb "chromiumos/tast/services/cros/firmware"
)

// CheckBootMode forwards to the CheckBootMode RPC to check whether the DUT is in a specified boot mode.
func CheckBootMode(ctx context.Context, utils fwpb.UtilsServiceClient, bootMode fwCommon.BootMode) (bool, error) {
	res, err := utils.CurrentBootMode(ctx, &empty.Empty{})
	if err != nil {
		return false, errors.Wrap(err, "calling CurrentBootMode rpc")
	}
	return bootMode == fwCommon.BootModeFromProto[res.BootMode], nil
}
