// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"os"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:    MlCoreEffectsPipeline,
		Desc:    "Validates that the ML Core bindings layer interacts correctly with the libcros_ml_core_internal.so library from G3",
		Timeout: 5 * time.Minute,
		Contacts: []string{
			"shafron@google.com",
		},
		Attr: []string{
			"group:mainline", "informational",
		},
		SoftwareDeps: []string{""},
		Params: []testing.Param{
			{
				Name: "effects_pipeline",
				Val:  []string{"libcros_ml_core_tast_effects_pipeline"},
			},
		},
	})
}

func MlCoreEffectsPipeline(ctx context.Context, s *testing.State) {
	cmdArgs := s.Param().([]string)

	cmd := testexec.CommandContext(ctx, cmdArgs[0], cmdArgs[1:]...)
	cmd.Env = append(os.Environ(), "ANDROID_LOG_TAGS=*:f")

	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		s.Error("Failed to run test suite: ", err)
	}
}
