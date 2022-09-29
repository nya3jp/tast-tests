// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package starfish provides functions for testing starfish module.
package starfish

import (
	"context"
	"time"

	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

// Helper fetches starfish module properties.
type Helper struct {
	sf *Starfish
}

// NewHelper creates a Helper object and ensures that a starfish module is present.
func NewHelper(ctx context.Context) (*Helper, error) {
	ctx, st := timing.Start(ctx, "Helper.NewHelper")
	defer st.End()
	var sfs Starfish
	helper := Helper{sf: &sfs}
	if err := helper.sf.Setup(ctx); err != nil {
		return nil, err
	}
	defer helper.sf.Teardown(ctx)
	if err := helper.sf.DeviceId(ctx); err != nil {
		return nil, err
	}
	if err := helper.sf.SimStatus(ctx); err != nil {
		return nil, err
	}
	if err := helper.sf.SimInsert(ctx, 5); err != nil {
		return nil, err
	}
	testing.Sleep(ctx, 3*time.Second)
	if err := helper.sf.SimEject(ctx); err != nil {
		return nil, err
	}
	return &helper, nil
}
