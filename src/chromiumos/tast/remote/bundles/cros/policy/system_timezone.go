// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"encoding/json"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	chwsec "chromiumos/tast/common/hwsec"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/hwsec"
	"chromiumos/tast/remote/policyutil"
	"chromiumos/tast/rpc"
	ps "chromiumos/tast/services/cros/policy"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SystemTimezone,
		Desc: "Behavior of SystemTimezone policy",
		Contacts: []string{
			"alexanderhartl@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.policy.PolicyService", "tast.cros.policy.SystemTimezoneService"},
		Timeout:      5 * time.Minute,
	})
}

func SystemTimezone(ctx context.Context, s *testing.State) {
	defer func(ctx context.Context) {
		if err := policyutil.EnsureTPMIsResetAndPowerwash(ctx, s.DUT()); err != nil {
			s.Error("Failed to reset TPM: ", err)
		}
	}(ctx)

	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Minute)
	defer cancel()

	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	pc := ps.NewPolicyServiceClient(cl.Conn)

	if _, err := pc.StartExternalDataServer(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to start a URLPolicyServer: ", err)
	}
	defer pc.StopExternalDataServer(ctx, &empty.Empty{})

	pb := fakedms.NewPolicyBlob()

	// Get the currently set timezone from the DUT to make sure we set a different timezone in the policy.
	psc := ps.NewSystemTimezoneServiceClient(cl.Conn)
	timezone := "CEST"
	ret, err := psc.GetSystemTimezone(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("Failed to get system timezone: ", err)
	}
	if ret.Timezone != timezone {
		pb.AddPolicy(&policy.SystemTimezone{
			Val: "Europe/Berlin",
		})
	} else {
		pb.AddPolicy(&policy.SystemTimezone{
			Val: "America/Los_Angeles",
		})
		timezone = "PDT"
	}

	pJSON, err := json.Marshal(pb)
	if err != nil {
		s.Fatal("Failed to serialize policies: ", err)
	}

	if _, err := pc.EnrollUsingChrome(ctx, &ps.EnrollUsingChromeRequest{
		PolicyJson: pJSON,
	}); err != nil {
		s.Fatal("Failed to enroll using chrome: ", err)
	}
	defer pc.StopChromeAndFakeDMS(ctx, &empty.Empty{})

	// Reboot the device to ensure the new timezone is set.
	r, err := hwsec.NewCmdRunner(s.DUT())
	if err != nil {
		s.Fatal("Failed to create CmdRunner: ", err)
	}

	utility, err := chwsec.NewUtilityCryptohomeBinary(r)
	if err != nil {
		s.Fatal("Failed to create UtilityCryptohomeBinary: ", err)
	}

	helper, err := hwsec.NewHelper(utility, r, s.DUT())
	if err != nil {
		s.Fatal("Failed to create Helper: ", err)
	}

	helper.Reboot(ctx)

	cl, err = rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	psc = ps.NewSystemTimezoneServiceClient(cl.Conn)

	// Check if the timezone on the DUT was set correctly.
	ret, err = psc.GetSystemTimezone(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("Failed to get system timezone: ", err)
	}
	if ret.Timezone != timezone {
		s.Fatal("Failed to set the SystemTimezone policy")
	}
}
