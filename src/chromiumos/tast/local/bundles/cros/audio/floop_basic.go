// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"context"

	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/audio/crastestclient"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: FloopBasic,
		Desc: "Chcek cras_test_client --request_floop_mask works",
		Contacts: []string{
			"aaronyu@google.com",
			"chromeos-audio-bugs@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
	})
}

func FloopBasic(ctx context.Context, s *testing.State) {
	if err := audio.RestartCras(ctx); err != nil {
		s.Fatal("Cannot restart CRAS: ", err)
	}

	dev1, err := crastestclient.RequestFloopMask(ctx, 1)
	if err != nil {
		s.Fatal("RequestFloopMask failed for mask=1: ", err)
	}

	// TODO(b/227449103): Test the same mask results in the same device id.

	dev2, err := crastestclient.RequestFloopMask(ctx, 2)
	if err != nil {
		s.Fatal("RequestFloopMask failed for mask=2: ", err)
	}

	if dev2 == dev1 {
		s.Errorf("consecutive RequestFloopMask with different masks returned the same device: mask=1 -> %d; mask=2 -> %d",
			dev1, dev2,
		)
	}
}
