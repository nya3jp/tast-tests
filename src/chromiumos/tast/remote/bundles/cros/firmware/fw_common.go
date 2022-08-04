// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"strings"

	fwUtils "chromiumos/tast/remote/bundles/cros/firmware/utils"
	"chromiumos/tast/common/firmware/bios"
	"chromiumos/tast/remote/firmware/fixture"
	pb "chromiumos/tast/services/cros/firmware"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: FWCommon,
		Desc: "Exercises critical services that are used by many FAFT tests to make keeping FAFT reliable easier",
		Contacts: []string{
			"kmshelton@chromium.org",         // Test author
			"cros-base-os-ep@google.com",     // Backup mailing list
		},
		// TODO(kmshelton): Move to firmware_smoke after firmware_unstable verification
		Attr:        []string{"group:firmware", "firmware_unstable"},
		Fixture:     fixture.NormalMode,
		ServiceDeps: []string{"tast.cros.firmware.BiosService", "tast.cros.firmware.UtilsService"},
	})
}

func FWCommon(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper

	if err := h.RequireBiosServiceClient(ctx); err != nil {
		s.Fatal("Requiring BiosServiceClient: ", err)
	}
	bs := h.BiosServiceClient



}
