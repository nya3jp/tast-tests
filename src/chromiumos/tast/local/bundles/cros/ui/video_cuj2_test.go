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

func TestVideoCUJ2Params(t *testing.T) {
	params := []cuj.TestParam{
		cuj.TestParam{
			Tier:            cuj.Basic,
			ApplicationName: "youtube",
			ApplicationType: cuj.Web,
			Timeout:         10 * time.Minute,
		},
		cuj.TestParam{
			Tier:            cuj.Plus,
			ApplicationName: "youtube",
			ApplicationType: cuj.Web,
			Timeout:         10 * time.Minute,
		},
		cuj.TestParam{
			Tier:            cuj.Basic,
			ApplicationName: "netflix",
			ApplicationType: cuj.Web,
			Timeout:         10 * time.Minute,
		},
		cuj.TestParam{
			Tier:            cuj.Plus,
			ApplicationName: "netflix",
			ApplicationType: cuj.Web,
			Timeout:         10 * time.Minute,
		},
		cuj.TestParam{
			Tier:            cuj.Basic,
			ApplicationName: "youtube",
			ApplicationType: cuj.APP,
			Timeout:         10 * time.Minute,
		},
		cuj.TestParam{
			Tier:            cuj.Plus,
			ApplicationName: "youtube",
			ApplicationType: cuj.APP,
			Timeout:         10 * time.Minute,
		},
	}
	genparams.Ensure(t, "video_cuj2.go", cuj.MakeCUJCaseParam(t, params))
}
