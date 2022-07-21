// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/rpc"
	ps "chromiumos/tast/services/cros/policy"
	"chromiumos/tast/testing"
)

type testZTEInfo struct {
	dmserver string // device management server url
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         GAIAZTEEnrollment,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "ZTE GAIA Enroll a device without checking policies",
		Contacts: []string{
			"rzakarian@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		Attr:         []string{"group:dpanel-end2end"},
		SoftwareDeps: []string{"reboot", "chrome"},
		ServiceDeps:  []string{"tast.cros.policy.PolicyService"},
		Timeout:      7 * time.Minute,
		Params: []testing.Param{
			{
				Name: "autopush",
				Val: testZTEInfo{
					dmserver: "https://crosman-alpha.sandbox.google.com/devicemanagement/data/api",
					//dmserver: "https://m.google.com/devicemanagement/data/api",
				},
			},
		},
		Vars: []string{"ui.signinProfileTestExtensionManifestKey"},
	})
}

func GAIAZTEEnrollment(ctx context.Context, s *testing.State) {
	param := s.Param().(testZTEInfo)
	dmServerURL := param.dmserver

	/*
		defer func(ctx context.Context) {
			if err := policyutil.EnsureTPMAndSystemStateAreResetRemote(ctx, s.DUT()); err != nil {
				s.Error("Failed to reset TPM after test: ", err)
			}
		}(ctx)
	*/

	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Minute)
	defer cancel()

	/*
		if err := policyutil.EnsureTPMAndSystemStateAreResetRemote(ctx, s.DUT()); err != nil {
			s.Fatal("Failed to reset TPM: ", err)
		}
	*/

	//if err := policyutil.ResetDeviceToFactoryStateForZTE(ctx, s.DUT()); err != nil {
	//	s.Fatal("Failed to reset TPM: ", err)
	//}

	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	pc := ps.NewPolicyServiceClient(cl.Conn)
	//testing.Sleep(ctx, 60*time.Second)

	if _, err := pc.GAIAZTEEnrollUsingChrome(ctx, &ps.GAIAZTEEnrollUsingChromeRequest{
		DmserverURL: dmServerURL,
		ManifestKey: s.RequiredVar("ui.signinProfileTestExtensionManifestKey"),
	}); err != nil {
		s.Fatal("Failed to ZTE enroll using chrome: ", err)
	}
	if _, err := pc.GAIAZTEEnrollClickThrough(ctx, &empty.Empty{}); err != nil {
		s.Error("Failed to click through ZTE: ", err)
	}
	testing.Sleep(ctx, 60*time.Second)
}
