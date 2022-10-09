// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"
	"reflect"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"chromiumos/tast/framework/protocol"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     LocalFixtureDUTFeature,
		Desc:     "Test to check whether local fixture can access DUT features",
		Contacts: []string{"seewaifu@chromium.org", "tast-owners@google.com"},
		Fixture:  "metaLocalFixtureDUTFeature",
	})
}

func LocalFixtureDUTFeature(ctx context.Context, s *testing.State) {
	wanted := s.Features("")
	got := s.FixtValue().(*protocol.DUTFeatures)
	allowUnexported := func(reflect.Type) bool { return true }
	if diff := cmp.Diff(got, wanted, cmpopts.EquateEmpty(), cmp.Exporter(allowUnexported)); diff != "" {
		s.Logf("Got unexpected feature from fixture (-got +want): %s", diff)
		s.Fatal("Got unexpected feature from fixture")
	}
}
