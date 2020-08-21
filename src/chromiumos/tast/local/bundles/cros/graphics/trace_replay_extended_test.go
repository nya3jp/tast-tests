// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

// To update test parameters after modifying this file, run:
// TAST_GENERATE_UPDATE=1 ~/trunk/src/platform/tast/tools/go.sh test -count=1 chromiumos/tast/local/bundles/cros/graphics/

// See src/chromiumos/tast/local/crostini/params.go for more documentation

import (
	"testing"
	"time"

	"chromiumos/tast/common/genparams"
	"chromiumos/tast/local/crostini"
)

func TestTraceReplayExtendedParams(t *testing.T) {
	params := crostini.MakeTestParamsFromList(t, []crostini.Param{
		{
			Name:    "glxgears_1minute",
			Timeout: 45 * time.Minute,
			Val: `comm.TestGroupConfig{
					Labels: []string{"short"},
					Repository: comm.RepositoryInfo{
						RootURL: "gs://chromiumos-test-assets-public/tast/cros/graphics/traces/repo",
						Version: 1,
					},
					ExtendedDuration: 1 * 60,
				}`,
			MinimalSet:    true,
			IsNotMainline: true,
		}})

	genparams.Ensure(t, "trace_replay_extended.go", params)
}
