// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package telemetryextension

import (
	"context"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Zzz,
		Desc: "Tests Zzz core functionality such as API, permissions, communication with PWA",
		Contacts: []string{
			"lamzin@google.com", // Test and Telemetry Extension author
			"mgawad@google.com", // Telemetry Extension author
			"cros-oem-services-team@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
	})
}

func Zzz(ctx context.Context, s *testing.State) {

}
