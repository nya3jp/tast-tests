// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package checkers

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/firmware"
	"chromiumos/tast/errors"
	pb "chromiumos/tast/services/cros/firmware"
)

// GBBFlags checks that the flags on DUT equals the wanted one.
// You must add `ServiceDeps: []string{"tast.cros.firmware.BiosService"}` to your `testing.Test` to use this.
func (c *Checker) GBBFlags(ctx context.Context, want pb.GBBFlagsState) error {
	if err := c.h.RequireBiosServiceClient(ctx); err != nil {
		return err
	}
	res, err := c.h.BiosServiceClient.GetGBBFlags(ctx, &empty.Empty{})
	if err != nil {
		return errors.Wrap(err, "could not get GBB flags")
	}
	if err = firmware.GBBFlagsVerifyExpected(want, *res); err != nil {
		return errors.Wrap(err, "GBB flags")
	}
	return nil
}
