// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"time"

	updateengine_test "chromiumos/tast/local/bundles/cros/platform/updateengine"
	"chromiumos/tast/local/updateengine"
	"chromiumos/tast/testing"
)

type featuresTestParam struct {
	feature updateengine.Feature
	enabled bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func: UpdateEngineFeatures,
		Desc: "Verifies that update engine features work as intended",
		Contacts: []string{
			"kimjae@chromium.org",
			"chromeos-core-services@google.com",
		},
		Fixture: "updateEngineReady",
		Attr:    []string{"group:mainline"},
		Timeout: 3 * time.Minute,
		Params: []testing.Param{
			{
				Name: "consumer_auto_update_enabled",
				Val: featuresTestParam{
					feature: updateengine.ConsumerAutoUpdate,
					enabled: true,
				},
				ExtraAttr: []string{"informational"},
			},
			{
				Name: "consumer_auto_update_disabled",
				Val: featuresTestParam{
					feature: updateengine.ConsumerAutoUpdate,
					enabled: false,
				},
				ExtraAttr: []string{"informational"},
			},
		},
	})
}

func UpdateEngineFeatures(ctx context.Context, s *testing.State) {
	ftParam := s.Param().(featuresTestParam)
	switch ftParam.feature {
	case updateengine.ConsumerAutoUpdate:
		if err := updateengine_test.ValidateConsumerAutoUpdate(ctx, ftParam.feature, ftParam.enabled); err != nil {
			s.Fatal("Failed to validate consumer auto update: ", err)
		}
	default:
		s.Fatal("Feature test is not implemented")
	}
}
