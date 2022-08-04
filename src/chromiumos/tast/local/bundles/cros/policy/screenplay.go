// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Screenplay,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "A screenplay test",
		Contacts: []string{
			"alexanderhartl@google.com", // Test author
		},
		Attr: []string{"group:mainline"},
		SearchFlags: []*testing.StringPair{
			&testing.StringPair{
				Key:   "feature_id",
				Value: "1",
			},
			&testing.StringPair{
				Key:   "feature_id",
				Value: "2",
			}},
	})
}

func Screenplay(ctx context.Context, s *testing.State) {
	s.Log("SCREENPLAY")
}
