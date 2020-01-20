// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/remote/hwsec"
	"chromiumos/tast/rpc"
	ps "chromiumos/tast/services/cros/policy"
	"chromiumos/tast/services/cros/wilco"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: APIGetConfigurationData,
		Desc: "Test sending GetConfigurationData gRPC request from Wilco DTC VM to the Wilco DTC Support Daemon daemon",
		Contacts: []string{
			"vsavu@chromium.org",  // Test author
			"pmoy@chromium.org",   // wilco_dtc_supportd author
			"lamzin@chromium.org", // wilco_dtc_supportd maintainer
			"chromeos-wilco@google.com",
		},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"reboot", "vm_host", "wilco", "chrome"},
		ServiceDeps:  []string{"tast.cros.wilco.WilcoService", "tast.cros.policy.PolicyService"},
	})
}

func APIGetConfigurationData(ctx context.Context, s *testing.State) {
	if err := hwsec.EnsureOwnerIsReset(ctx, s.DUT()); err != nil {
		s.Fatal("Failed to reset TPM: ", err)
	}

	defer func() {
		if err := hwsec.EnsureOwnerIsReset(ctx, s.DUT()); err != nil {
			s.Fatal("Failed to reset TPM: ", err)
		}
	}()

	configurationData := `{"test": 1}`

	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	pc := ps.NewPolicyServiceClient(cl.Conn)

	if _, err := pc.StartURLPolicyServer(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to start a URLPolicyServer: ", err)
	}

	res, err := pc.ServeURLPolicy(ctx, &ps.ServeURLPolicyRequest{
		Contents: []byte(configurationData),
	})
	if err != nil {
		s.Fatal("Failed to serve policy: ", err)
	}
	defer pc.StopURLPolicyServer(ctx, &empty.Empty{})

	pb := fakedms.NewPolicyBlob()
	pb.AddPolicy(&policy.DeviceWilcoDtcAllowed{Val: true})
	pb.AddPolicy(&policy.DeviceWilcoDtcConfiguration{
		Val: &policy.DeviceWilcoDtcConfigurationValue{
			Url:  res.Url,
			Hash: string(res.Hash),
		},
	})

	pJSON, err := pb.ToRawJSON()
	if err != nil {
		s.Fatal("Failed to serialize policies: ", err)
	}

	req := ps.PolicyBlob{
		PolicyBlob: pJSON,
	}

	if _, err := pc.EnrollUsingChrome(ctx, &req); err != nil {
		s.Fatal("Failed to enroll using chrome: ", err)
	}
	defer pc.StopChrome(ctx, &empty.Empty{})

	wc := wilco.NewWilcoServiceClient(cl.Conn)

	if status, err := wc.GetStatus(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Could not get running status: ", err)
	} else if !status.WilcoDtcSupportdRunning {
		s.Fatal("Wilco DTC Supportd not running")
	}

	data, err := wc.GetConfigurationData(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("Failed to perform GetConfigurationData: ", err)
	}

	if data.JsonConfigurationData != configurationData {
		s.Errorf("Unexpected policy value: Got %s, Want %s", data.JsonConfigurationData, configurationData)
	}
}
