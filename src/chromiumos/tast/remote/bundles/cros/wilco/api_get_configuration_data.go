// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/remote/hwsec"
	"chromiumos/tast/rpc"
	ps "chromiumos/tast/services/cros/policy"
	"chromiumos/tast/services/cros/wilco"
	wpb "chromiumos/tast/services/cros/wilco"
	"chromiumos/tast/testing"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/common/policy"
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
	hwsec.EnsureTPMIsReset(ctx, s.DUT())

	func() {
		cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
		if err != nil {
			s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
		}
		defer cl.Close(ctx)

		wc := wilco.NewWilcoServiceClient(cl.Conn)

		if _, err := wc.Start(ctx, &wpb.RunConfiguration{
			StartWilcoDtcSupportd: true, StartWilcoDtc: true,
		}); err != nil {
			s.Fatal("Failed to start wilco VM: ", err)
		}
		defer wc.Stop(ctx, &empty.Empty{})

		data, err := wc.GetConfigurationData(ctx, &empty.Empty{})
		if err != nil {
			s.Fatal("Failed to perform GetConfigurationData: ", err)
		}

		if data.JsonConfigurationData != "" {
			s.Error("Expected data to be empty when not enrolled")
		}
	}()

	hwsec.EnsureTPMIsReset(ctx, s.DUT())

	func() {
		configurationData := `{"test": 1}`

		cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
		if err != nil {
			s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
		}
		defer cl.Close(ctx)

		ec := ps.NewPolicyServiceClient(cl.Conn)

		res, err := ec.StartURLPolicyServer(ctx, &ps.StartURLPolicyServerRequest {
			Contents: []byte(configurationData),
		})
		if err != nil {
			s.Fatal("Failed to start a URLPolicyServer: ", err)
		}
		defer ec.StopURLPolicyServer(ctx, &ps.StopURLPolicyServerRequest{
			Url: res.Url,
		})

		pb := fakedms.NewPolicyBlob()
		pb.AddPolicy(&policy.DeviceWilcoDtcAllowed{Val: true, Stat: policy.StatusSet})
		pb.AddPolicy(&policy.DeviceWilcoDtcConfiguration{
			Val: &policy.DeviceWilcoDtcConfigurationValue {
				Url: res.Url,
				Hash: string(res.Hash),
			}, Stat: policy.StatusSet,
		})

		pJSON, err := pb.ToRawJSON()
		if err != nil {
			s.Fatal("Failed to serialize policies: ", err)
		}

		req := ps.PolicyBlob {
			PolicyBlob: pJSON,
		}

		if _, err := ec.EnrollUsingChrome(ctx, &req); err != nil {
			s.Fatal("Failed to enroll using chrome: ", err)
		}

		wc := wilco.NewWilcoServiceClient(cl.Conn)

		/*
		if _, err := wc.Start(ctx, &wpb.RunConfiguration{
			StartWilcoDtcSupportd: true, StartWilcoDtc: true,
		}); err != nil {
			s.Fatal("Failed to start wilco VM: ", err)
		}
		defer wc.Stop(ctx, &empty.Empty{})
		*/

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
	}()
}
