// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

/*
This file implements functions to check or switch the DUT's boot mode.
*/

import (
	"context"

	fwCommon "chromiumos/tast/common/firmware"
	"chromiumos/tast/errors"
	fwpb "chromiumos/tast/services/cros/firmware"
)

// CheckBootMode forwards to the CheckBootMode RPC to check whether the DUT is in a specified boot mode.
func CheckBootMode(ctx context.Context, utils fwpb.UtilsServiceClient, bootMode fwCommon.BootMode) (bool, error) {
	req := &fwpb.CheckBootModeRequest{BootMode: fwCommon.ProtoBootMode[bootMode]}
	res, err := utils.CheckBootMode(ctx, req)
	if err != nil {
		return false, errors.Wrapf(err, "calling fwpb.CheckBootMode with bootMode=%s", bootMode)
	}
	return res.GetVerified(), nil
}
