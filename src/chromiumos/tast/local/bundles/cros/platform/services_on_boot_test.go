// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestParseServiceFailures(t *testing.T) {
	got, err := parseServiceFailures(`
[    1.374461] init: early-failure main process (315) terminated with status 124
[    9.131824] init: failsafe-delay main process (944) killed by TERM signal
[12345.678901] init: sound_card_init main process (3343) killed by ABRT signal
`)
	if err != nil {
		t.Fatal("parseServiceFailures failed")
	}

	want := []serviceFailure{
		{
			Message:     "[    1.374461] init: early-failure main process (315) terminated with status 124",
			JobName:     "early-failure",
			ProcessName: "main",
			ExitStatus:  124,
		},
		{
			Message:        "[    9.131824] init: failsafe-delay main process (944) killed by TERM signal",
			JobName:        "failsafe-delay",
			ProcessName:    "main",
			ExitStatus:     -1,
			KilledBySignal: "TERM",
		},
		{
			Message:        "[12345.678901] init: sound_card_init main process (3343) killed by ABRT signal",
			JobName:        "sound_card_init",
			ProcessName:    "main",
			ExitStatus:     -1,
			KilledBySignal: "ABRT",
		},
	}

	if diff := cmp.Diff(want, got, cmp.AllowUnexported(serviceFailure{})); diff != "" {
		t.Fatalf("parseServiceFailures return unexpected output: -want; +got\n%s", diff)
	}
}
