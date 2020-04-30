// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package utils

import (
	"context"

	"cienet.com/cats/node/sdk"
	"cienet.com/cats/node/sdk/ui"

	"chromiumos/tast/common/mtbferrors"
)

// ClickSelector finds selector and click it if it exists.
func ClickSelector(ctx context.Context, dut *Device, selector string) error {
	if isExist, err := dut.Client.UIAObjEventWait(dut.DeviceID, selector, 1000, ui.ObjEventTypeAppear).Do(ctx); err != nil {
		return err
	} else if isExist {
		dut.Client.UIAClick(dut.DeviceID).Selector(selector).Do(ctx)
	}
	return nil
}

// FailCase calls the node SDK Fail() function to fail a test case.
func FailCase(ctx context.Context, client sdk.DelegateClient, err error) error {

	if mtbferr, ok := err.(mtbferrors.MTBFError); ok {
		return client.Fail(ctx, "", true, uint32(mtbferr.ErrorCode()), mtbferr.Error())
	}
	return client.Fail(ctx, err.Error(), true, 0, "")
}
