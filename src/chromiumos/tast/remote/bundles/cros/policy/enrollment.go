// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"encoding/json"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/policyutil"
	"chromiumos/tast/rpc"
	ps "chromiumos/tast/services/cros/policy"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Enrollment,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Enroll a device without checking policies",
		Contacts: []string{
			"vsavu@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		Attr:         []string{"group:enrollment"},
		SoftwareDeps: []string{"reboot", "chrome"},
		ServiceDeps:  []string{"tast.cros.policy.PolicyService"},
		Timeout:      7 * time.Minute,
	})
}

func Enrollment(ctx context.Context, s *testing.State) {
	defer func(ctx context.Context) {
		if err := policyutil.EnsureTPMAndSystemStateAreReset(ctx, s.DUT()); err != nil {
			s.Error("Failed to reset TPM after test: ", err)
		}
	}(ctx)

	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Minute)
	defer cancel()

	if err := policyutil.EnsureTPMAndSystemStateAreReset(ctx, s.DUT()); err != nil {
		s.Fatal("Failed to reset TPM: ", err)
	}

	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	pJSON, err := json.Marshal(fakedms.NewPolicyBlob())
	if err != nil {
		s.Fatal("Failed to serialize policies: ", err)
	}

	pc := ps.NewPolicyServiceClient(cl.Conn)

	if _, err := pc.EnrollUsingChrome(ctx, &ps.EnrollUsingChromeRequest{
		PolicyJson: pJSON,
	}); err != nil {
		s.Fatal("Failed to enroll using chrome: ", err)
	}
	defer pc.StopChromeAndFakeDMS(ctx, &empty.Empty{})
}
