// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"context"
	"encoding/json"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/bundles/cros/wilco/wilcoextension"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/local/wilco"
	"chromiumos/tast/testing"
	dtcpb "chromiumos/wilco_dtc"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: APISendMessageToUIEnrolled,
		Desc: "Test sending a message from the Wilco DTC VM to the Chromium extension",
		Contacts: []string{
			"vsavu@chromium.org",       // Test author
			"bisakhmondal00@gmail.com", // Supported test author
			"pmoy@chromium.org",        // wilco_dtc_supportd author
			"lamzin@google.com",        // wilco_dtc_supportd maintainer
			"chromeos-wilco@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"vm_host", "wilco", "chrome"},
		Timeout:      10 * time.Minute,
		Fixture:      "wilcoDTCEnrolledExtensionSupport",
	})
}

// APISendMessageToUIEnrolled tests Wilco DTC SendMessageToUi gRPC API.
func APISendMessageToUIEnrolled(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fdms := s.FixtValue().(*fixtures.FixtData).FakeDMS

	pb := fakedms.NewPolicyBlob()
	// wilco_dtc and wilco_dtc_supportd only run for affiliated users.
	pb.DeviceAffiliationIds = []string{"default_affiliation_id"}
	pb.UserAffiliationIds = []string{"default_affiliation_id"}

	// After this point, IsUserAffiliated flag should be updated.
	if err := policyutil.ServeBlobAndRefresh(ctx, fdms, cr, pb); err != nil {
		s.Fatal("Failed to serve and refresh: ", err)
	}

	// We should add policy value in the middle of 2 ServeBlobAndRefresh calls to be sure
	// that IsUserAffiliated flag is updated and policy handler is triggered.
	pb.AddPolicy(&policy.DeviceWilcoDtcAllowed{Val: true})

	// After this point, the policy handler should be triggered.
	if err := policyutil.ServeBlobAndRefresh(ctx, fdms, cr, pb); err != nil {
		s.Fatal("Failed to serve and refresh: ", err)
	}

	conn, err := wilcoextension.NewConnectionToWilcoExtension(ctx, cr)
	if err != nil {
		s.Fatal("Failed to create connection to extension: ", err)
	}
	defer conn.Close()

	if err := conn.CreatePort(ctx); err != nil {
		s.Fatal("Failed to create port to built-in application: ", err)
	}
	if err := conn.StartListener(ctx); err != nil {
		s.Fatal("Failed to start listener: ", err)
	}

	type testMsg struct {
		Test int
	}

	uiResponse := testMsg{
		Test: 8,
	}

	if err := conn.AddReply(ctx, &uiResponse); err != nil {
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
	request := dtcpb.SendMessageToUiRequest{
		JsonMessage: string(marshaled),
	}
	response := dtcpb.SendMessageToUiResponse{}

	if err := wilco.DPSLSendMessage(ctx, "SendMessageToUi", &request, &response); err != nil {
		s.Error("Failed to send message to UI: ", err)
	}
	var vmResponse testMsg
	if err := json.Unmarshal([]byte(response.ResponseJsonMessage), &vmResponse); err != nil {
		s.Logf("Response JSON message: %q", response.ResponseJsonMessage)
		s.Fatal("Failed to unmarshal message: ", err)
	}

	if uiResponse != vmResponse {
		s.Errorf("Unexpected reply received: got %v; want %v", vmResponse, uiResponse)
	}

	s.Log("Waiting for message")
	var uiRequest testMsg
	if err := conn.WaitForMessage(ctx, &uiRequest); err != nil {
		s.Fatal("Failed to send message using extension: ", err)
	}

	if vmRequest != uiRequest {
		s.Errorf("Unexpected request received: got %v; want %v", uiRequest, vmRequest)
	}
}
