// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"chromiumos/tast/common/tape"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/exec"
	"chromiumos/tast/remote/gaiaenrollment"
	"chromiumos/tast/remote/policyutil"
	"chromiumos/tast/rpc"
	ps "chromiumos/tast/services/cros/policy"
	"chromiumos/tast/testing"
)

const gaiaZTEEnrollmentTimeout = 7 * time.Minute

func init() {
	testing.AddTest(&testing.Test{
		Func:         GAIAZTEEnrollment,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "ZTE GAIA Enroll a device without checking policies",
		Contacts: []string{
			"rzakarian@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		Attr:         []string{"group:dmserver-zteenrollment-daily"},
		SoftwareDeps: []string{"reboot", "chrome"},
		ServiceDeps:  []string{"tast.cros.policy.PolicyService", "tast.cros.tape.Service", "tast.cros.hwsec.OwnershipService"},
		Timeout:      7 * time.Minute,
		SearchFlags: []*testing.StringPair{{
			Key: "feature_id",
			// ZTE Enrollment.
			Value: "screenplay-6f0905f0-9ecd-4974-b4a1-7e4b828b5dc2",
		}},
		Params: []testing.Param{
			{
				Name: "autopush",
				Val: gaiaenrollment.TestParams{
					DMServer:             "https://crosman-alpha.sandbox.google.com/devicemanagement/data/api",
					PoolID:               tape.ZTETestAutomation,
					SerialNumber:         "policy.GAIAZTEEnrollment.serial_number",
					HardwareModel:        "policy.GAIAZTEEnrollment.hardware_model",
					DeviceProvisionToken: "policy.GAIAZTEEnrollment.device_provision_token",
					CustomerID:           "policy.GAIAZTEEnrollment.customer_id",
					BatchKey:             "policy.GAIAZTEEnrollment.batch_key",
				},
			},
		},
		Vars: []string{
			"ui.signinProfileTestExtensionManifestKey",
			tape.ServiceAccountVar,
			"policy.GAIAZTEEnrollment.serial_number",
			"policy.GAIAZTEEnrollment.hardware_model",
			"policy.GAIAZTEEnrollment.device_provision_token",
			"policy.GAIAZTEEnrollment.customer_id",
			"policy.GAIAZTEEnrollment.batch_key",
		},
	})
}

func GAIAZTEEnrollment(ctx context.Context, s *testing.State) {
	param := s.Param().(gaiaenrollment.TestParams)
	dmServerURL := param.DMServer
	poolID := param.PoolID
	serialNumber := s.RequiredVar(param.SerialNumber)
	hardwareModel := s.RequiredVar(param.HardwareModel)
	deviceProvisionToken := s.RequiredVar(param.DeviceProvisionToken)
	customerID := s.RequiredVar(param.CustomerID)
	batchKey := s.RequiredVar(param.BatchKey)

	// The block below pre-provisions the device for the next run.
	bodyCommand := fmt.Sprintf("{\"requests\": [ {\"preProvisionedDevice\":{serialNumber:\"%s\",hardwareModel:\"%s\",devicePreProvisioningToken:\"%s\",attestedDeviceId:\"%s\",customer_id:\"%s\"}}]}", serialNumber, hardwareModel, deviceProvisionToken, serialNumber, customerID)
	urlWithBatchKey := fmt.Sprintf("https://chromecommercial.googleapis.com/v1/preProvisionedDevices:batchCreate?key=%s", batchKey)
	body := strings.NewReader(bodyCommand)
	req, err := http.NewRequest("POST", urlWithBatchKey, body)
	if err != nil {
		s.Fatal("Failed to create pre-provision request: ", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		s.Fatal("Failed to pre-provision device: ", err)
	}
	defer resp.Body.Close()

	dutConn := s.DUT().Conn()
	// Getting the date for 1 month ago, this is needed for the RLZ command below.
	currentTime := time.Now()
	last1Month := currentTime.AddDate(0, -1, 0)
	timeLayout := "2006-01-02"
	oneMonthAgoDate := last1Month.Format(timeLayout)
	// Setting the RLZ ping embargo end date.
	oneMonthAgo := fmt.Sprintf("rlz_embargo_end_date=\"%s\"", oneMonthAgoDate)
	if err := dutConn.CommandContext(ctx, "vpd", "-i", "RW_VPD", "-s", oneMonthAgo).Run(exec.DumpLogOnError); err != nil {
		s.Fatal("Failed to set rlz date: ", err)
	}

	defer func(ctx context.Context) {
		if err := policyutil.EnsureTPMAndSystemStateAreResetRemote(ctx, s.DUT()); err != nil {
			s.Error("Failed to reset TPM after test: ", err)
		}
	}(ctx)

	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Minute)
	defer cancel()

	if err := policyutil.EnsureTPMAndSystemStateAreResetRemote(ctx, s.DUT()); err != nil {
		s.Fatal("Failed to reset TPM: ", err)
	}

	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	pc := ps.NewPolicyServiceClient(cl.Conn)

	tapeClient, err := tape.NewClient(ctx, []byte(s.RequiredVar(tape.ServiceAccountVar)))
	if err != nil {
		s.Fatal("Failed to create tape client: ", err)
	}
	timeout := int32(gaiaZTEEnrollmentTimeout.Seconds())
	// Create an account manager and lease a test account for the duration of the test.
	accManager, acc, err := tape.NewOwnedTestAccountManagerFromClient(ctx, tapeClient, false /*lock*/, tape.WithTimeout(timeout), tape.WithPoolID(poolID))
	if err != nil {
		s.Fatal("Failed to create an account manager and lease an account: ", err)
	}
	defer accManager.CleanUp(ctx)

	if _, err := pc.GAIAZTEEnrollUsingChrome(ctx, &ps.GAIAZTEEnrollUsingChromeRequest{
		DmserverURL: dmServerURL,
		ManifestKey: s.RequiredVar("ui.signinProfileTestExtensionManifestKey"),
	}); err != nil {
		s.Fatal("Failed to ZTE enroll using chrome: ", err)
	}

	// Deprovision the DUT at the end of the test.
	defer func(ctx context.Context) {
		if err := tapeClient.DeprovisionHelper(ctx, cl, acc.CustomerID); err != nil {
			s.Fatal("Failed to deprovision device: ", err)
		}
	}(ctx)
}
