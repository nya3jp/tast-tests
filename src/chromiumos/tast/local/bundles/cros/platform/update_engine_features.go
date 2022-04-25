// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	updateengine_test "chromiumos/tast/local/bundles/cros/platform/updateengine"
	"chromiumos/tast/local/updateengine"
	"chromiumos/tast/testing"
)

type featuresTestParam struct {
	feature updateengine.Feature
}

func init() {
	testing.AddTest(&testing.Test{
		Func: UpdateEngineFeatures,
		Desc: "Verifies that update engine consumer auto update feature works",
		Contacts: []string{
			"kimjae@chromium.org",
			"chromeos-core-services@google.com",
		},
		Attr:    []string{"group:mainline"},
		Timeout: 3 * time.Minute,
		Params: []testing.Param{
			{
				Name: "consumer_auto_update",
				Val: featuresTestParam{
					feature: updateengine.ConsumerAutoUpdate,
				},
				ExtraAttr: []string{"informational"},
			},
		},
	})
}

func UpdateEngineFeatures(ctx context.Context, s *testing.State) {
	if err := testFeature(ctx, s.Param().(featuresTestParam).feature); err != nil {
		s.Fatal("Failed to test feature: ", err)
	}
}

func testFeature(ctx context.Context, feature updateengine.Feature) error {
	switch feature {
	case updateengine.ConsumerAutoUpdate:
		return updateengine_test.ValidateConsumerAutoUpdate(ctx, feature)
	default:
		return errors.New("Feature test not implemented")
	}
}
