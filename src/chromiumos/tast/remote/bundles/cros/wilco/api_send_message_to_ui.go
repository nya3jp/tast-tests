// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"context"
	"encoding/json"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/bundles/cros/wilco/wilcoextension"
	"chromiumos/tast/remote/policyutil"
	"chromiumos/tast/rpc"
	ps "chromiumos/tast/services/cros/policy"
	"chromiumos/tast/services/cros/wilco"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         APISendMessageToUI,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test sending a message from the Wilco DTC VM to the Chromium extension",
		Contacts: []string{
			"chromeos-oem-services@google.com", // Use team email for tickets.
			"bkersting@google.com",
			"lamzin@google.com",
		},
		Attr:         []string{"group:enrollment"},
		SoftwareDeps: []string{"reboot", "vm_host", "wilco", "chrome"},
		ServiceDeps: []string{
			"tast.cros.hwsec.OwnershipService",
			"tast.cros.policy.PolicyService",
			"tast.cros.wilco.WilcoService",
		},
		Timeout: 10 * time.Minute,
	})
}

// APISendMessageToUI tests Wilco DTC SendMessageToUi gRPC API.
// TODO(b/189457904): remove once wilco.APISendMessageToUIEnrolled will be stable enough.
func APISendMessageToUI(ctx context.Context, s *testing.State) { // NOLINT
	defer func(ctx context.Context) {
		if err := policyutil.EnsureTPMAndSystemStateAreReset(ctx, s.DUT(), s.RPCHint()); err != nil {
			s.Error("Failed to reset TPM: ", err)
		}
	}(ctx)

	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Minute)
	defer cancel()

	if err := policyutil.EnsureTPMAndSystemStateAreReset(ctx, s.DUT(), s.RPCHint()); err != nil {
		s.Fatal("Failed to reset TPM: ", err)
	}

	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	wc := wilco.NewWilcoServiceClient(cl.Conn)
	pc := ps.NewPolicyServiceClient(cl.Conn)

	pb := policy.NewBlob()
	pb.AddPolicy(&policy.DeviceWilcoDtcAllowed{Val: true})
	// wilco_dtc and wilco_dtc_supportd only run for affiliated users
	pb.DeviceAffiliationIds = []string{"default"}
	pb.UserAffiliationIds = []string{"default"}

	pJSON, err := json.Marshal(pb)
	if err != nil {
		s.Fatal("Failed to serialize policies: ", err)
	}

	if _, err := pc.EnrollUsingChrome(ctx, &ps.EnrollUsingChromeRequest{
		PolicyJson: pJSON,
		Extensions: []*ps.Extension{{
			Id: wilcoextension.ID,
			Files: []*ps.ExtensionFile{
				{
					Name:     "manifest.json",
					Contents: []byte(wilcoextension.Manifest),
				},
				{
					Name:     "background.js",
					Contents: []byte{},
				},
			},
		}},
	}); err != nil {
		s.Fatal("Failed to enroll using chrome: ", err)
	}
	defer pc.StopChromeAndFakeDMS(ctx, &empty.Empty{})

	nm, err := wilcoextension.NewBuiltInMessaging(ctx, pc)
	if err != nil {
		s.Fatal("Failed to start built-in messaging: ", err)
	}

	if err := nm.StartListener(ctx); err != nil {
		s.Fatal("Failed to start listener: ", err)
	}

	type testMsg struct {
		Test int
	}

	uiResponse := testMsg{
		Test: 8,
	}

	if err := nm.AddReply(ctx, &uiResponse); err != nil {
		s.Fatal("Failed to set reply: ", err)
	}

	vmRequest := testMsg{
		Test: 5,
	}

	marshaled, err := json.Marshal(vmRequest)
	if err != nil {
		s.Fatal("Failed to marshal message: ", err)
	}

	s.Log("Sending message to extension")
	reply, err := wc.SendMessageToUi(ctx, &wilco.SendMessageToUiRequest{
		JsonMessage: string(marshaled),
	})
	if err != nil {
		s.Fatal("Failed to perform SendMessageToUi: ", err)
	}

	var vmResponse testMsg
	if err := json.Unmarshal([]byte(reply.ResponseJsonMessage), &vmResponse); err != nil {
		s.Logf("Response JSON message: %q", reply.ResponseJsonMessage)
		s.Fatal("Failed to unmarshal message: ", err)
	}

	if uiResponse != vmResponse {
		s.Errorf("Unexpected reply received: got %v; want %v", vmResponse, uiResponse)
	}

	s.Log("Waiting for message")
	var uiRequest testMsg
	if err := nm.WaitForMessage(ctx, &uiRequest); err != nil {
		s.Fatal("Failed to send message using extension: ", err)
	}

	if vmRequest != uiRequest {
		s.Errorf("Unexpected request received: got %v; want %v", uiRequest, vmRequest)
	}
}
