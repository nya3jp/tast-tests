// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"context"
	"encoding/json"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
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
		Func: APISendMessageToUI,
		Desc: "Test sending a message from the Wilco DTC VM to the Chromium extension",
		Contacts: []string{
			"vsavu@chromium.org",  // Test author
			"pmoy@chromium.org",   // wilco_dtc_supportd author
			"lamzin@chromium.org", // wilco_dtc_supportd maintainer
			"chromeos-wilco@google.com",
		},
		Attr:         []string{"group:enrollment"},
		SoftwareDeps: []string{"reboot", "vm_host", "wilco", "chrome"},
		ServiceDeps:  []string{"tast.cros.wilco.WilcoService", "tast.cros.policy.PolicyService"},
		Timeout:      10 * time.Minute,
	})
}

func APISendMessageToUI(ctx context.Context, s *testing.State) { // NOLINT
	defer func(ctx context.Context) {
		if err := policyutil.EnsureTPMIsResetAndPowerwash(ctx, s.DUT()); err != nil {
			s.Error("Failed to reset TPM: ", err)
		}
	}(ctx)

	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Minute)
	defer cancel()

	if err := policyutil.EnsureTPMIsResetAndPowerwash(ctx, s.DUT()); err != nil {
		s.Fatal("Failed to reset TPM: ", err)
	}

	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	wc := wilco.NewWilcoServiceClient(cl.Conn)
	pc := ps.NewPolicyServiceClient(cl.Conn)

	pb := fakedms.NewPolicyBlob()
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
		Extensions: []*ps.Extension{&ps.Extension{
			Id: wilcoextension.ID,
			Files: []*ps.ExtensionFile{
				&ps.ExtensionFile{
					Name:     "manifest.json",
					Contents: []byte(wilcoextension.Manifest),
				},
				&ps.ExtensionFile{
					Name:     "background.js",
					Contents: []byte{},
				},
			},
		}},
	}); err != nil {
		s.Fatal("Failed to enroll using chrome: ", err)
	}
	defer pc.StopChromeAndFakeDMS(ctx, &empty.Empty{})

	nm, err := wilcoextension.NewNativeMessaging(ctx, pc)
	if err != nil {
		s.Fatal("Failed to start native messaging: ", err)
	}

	if err := nm.StartListener(ctx); err != nil {
		s.Fatal("Failed to start listener: ", err)
	}

	type testMsg struct {
		Test int
	}

	sendMsg := testMsg{
		Test: 5,
	}

	marshaled, err := json.Marshal(sendMsg)
	if err != nil {
		s.Fatal("Failed to marshal message: ", err)
	}

	if _, err := wc.SendMessageToUi(ctx, &wilco.SendMessageToUiRequest{
		JsonMessage: string(marshaled),
	}); err != nil {
		s.Fatal("Failed to perform SendMessageToUi: ", err)
	}

	s.Log("Waiting for message")
	var recvMsg testMsg
	if err := nm.WaitForMessage(ctx, &recvMsg); err != nil {
		s.Fatal("Failed to send message using extension: ", err)
	}

	if sendMsg != recvMsg {
		s.Errorf("Unexpected message received: got %v; want %v", recvMsg, sendMsg)
	}
}
