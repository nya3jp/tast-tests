// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

// Refer to cuj/test_params.go for the details of how to use this unit test.

import (
	"testing"
	"time"

	"chromiumos/tast/common/genparams"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
)

func TestEverydayMultiTaskingCUJParams(t *testing.T) {
	params := []cuj.TestParam{
		cuj.TestParam{
			Tier:            cuj.Basic,
			EnableBT:        false,
			ApplicationName: "ytmusic",
			Timeout:         10 * time.Minute,
		},
		cuj.TestParam{
			Tier:            cuj.Basic,
			EnableBT:        true,
			ApplicationName: "ytmusic",
			Timeout:         10 * time.Minute,
		},
		cuj.TestParam{
			Tier:            cuj.Basic,
			EnableBT:        true,
			ApplicationName: "spotify",
			Timeout:         10 * time.Minute,
		},
		cuj.TestParam{
			Tier:            cuj.Plus,
			EnableBT:        false,
			ApplicationName: "ytmusic",
			Timeout:         15 * time.Minute,
		},
		cuj.TestParam{
			Tier:            cuj.Plus,
			EnableBT:        true,
			ApplicationName: "ytmusic",
			Timeout:         15 * time.Minute,
		},
		cuj.TestParam{
			Tier:            cuj.Plus,
			EnableBT:        true,
			ApplicationName: "spotify",
			Timeout:         15 * time.Minute,
		},
	}
	genparams.Ensure(t, "everyday_multi_tasking_cuj.go", cuj.MakeCUJCaseParam(t, params))
}
