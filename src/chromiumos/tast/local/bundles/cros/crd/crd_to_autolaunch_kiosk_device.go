// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crd

// DoNotPush example to start
import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CrdToAutolaunchKioskDevice,
		Desc: "Checks that we can start a remote CRD connection to an auto-launched kiosk device",
		Contacts: []string{
			"jeroendh@google.com", // Test author
			"chromeos-commercial-crd@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "fakeDMS",
		// Fixture: "kioskLoggedIn",
	})
}

func CrdToAutolaunchKioskDevice(ctx context.Context, s *testing.State) {
	fdms, ok := s.FixtValue().(*fakedms.FakeDMS)
	if !ok {
		s.Fatal("Parent is not a FakeDMS fixture")
	}

	s.Log("DoNotPush Pre starting chrome")

	// DoNotPush in a fixture?
	// DoNotPush this test doesn't correctly reset the device afterwards.
	opts := []chrome.Option{
		chrome.GAIAEnterpriseEnroll(chrome.Creds{User: "jeroendh.minion@managedchrome.com", Pass: "duikbuilkuil"}),
		chrome.GAIALogin(chrome.Creds{User: "jeroendh.minion@managedchrome.com", Pass: "duikbuilkuil"}),
		chrome.DMSPolicy(fdms.URL),
		chrome.CustomLoginTimeout(chrome.ManagedUserLoginTimeout),
		//chrome.DeferLogin(),
	}

	_, err := chrome.New(ctx, opts...)
	if err != nil {
		s.Fatal("Chrome startup failed: ", err)
	}

	s.Log("DoNotPush Chrome startup finished")

}

// DoNotPush old version that uses the "kioskLoggedIn" fixture (and doesn't work).
// CrdToAutolaunchKioskDevice will launch an auto-launched kiosk device,
// and try to start a CRD connection to the device (through the remote command
// infrastructure).
func CrdToAutolaunchKioskDevice_old(ctx context.Context, s *testing.State) {
	const kCommandId = 111

	fd := s.FixtValue().(*fixtures.FixtData)
	fdms := fd.FakeDMS
	cr := fd.Chrome

	command := fakedms.NewRemoteCommand(fakedms.DEVICE_START_CRD_SESSION)
	command.CommandId = kCommandId
	command.Payload["IdlenessCutoffSec"] = 222

	// Send the remote command to the fake DMS server.
	pb := fakedms.NewPolicyBlob()
	pb.AddRemoteCommand(command)
	// DoNotPush , obviously use the correct address here (which?)
	pb.ServiceAccountIdentity = "055772cab2e614f3dd6676cde1b9bda1_5376930786@chrome-enterprise-devices.gserviceaccount.com"

	s.Log("DoNotPush Writing ServiceAccountIdentity")
	if err := fdms.WritePolicyBlob(pb); err != nil {
		s.Fatal("Failed to write policies to FakeDMS: ", err)
	}

	s.Log("DoNotPush Trying to force the chromebook to refresh the device policies")

	// DoNotPush this doesn't work at this time (and crashes). We need another way to refresh the service account id!
	// if err := policyutil.RefreshChromePolicies(ctx, cr); err != nil {
	// 	s.Fatal("Failed to refresh device policies: ", err)
	// }

	s.Log("DoNotPush Trying to force the chromebook to fetch remote commands")

	// DoNotPush helper
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create TestAPI connection: ", err)
	}

	if err := tconn.Eval(ctx, `tast.promisify(chrome.autotestPrivate.fetchDeviceRemoteCommands)()`, nil); err != nil {
		s.Fatal("Failed to fetch device remote commands: ", err)
	}

	s.Log("DoNotPush complete, now we wait")
	// // Start a new chrome
	// cr, err := chrome.New(ctx,
	// 	chrome.ARCEnabled(),
	// 	chrome.FakeLogin(chrome.Creds{User: fixtures.Username, Pass: fixtures.Password}),
	// 	chrome.DMSPolicy(fdms.URL))
	// if err != nil {
	// 	s.Fatal("Chrome login failed: ", err)
	// }
	// defer cr.Close(ctx)

	result, err := WaitForResult(ctx, fdms, kCommandId)
	if err != nil {
		s.Fatal(err)
	}

	if result.Result != fakedms.RESULT_SUCCESS {
		str, _ := json.MarshalIndent(result, "    ", "    ")
		s.Fatal("Remote command did not finish successfully. Response: " + string(str))
	}

	s.Log("DoNotPush Finished with great success")
}

// WaitForResult will wait until the fake DM server returns the result of the
// remote command with the given command ID.
func WaitForResult(ctx context.Context, fdms *fakedms.FakeDMS, command_id int) (*fakedms.RemoteCommandResponse, error) {
	var result fakedms.RemoteCommandResponse

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		responses, err := fdms.GetRemoteCommandResult()
		if err != nil {
			return testing.PollBreak(err)
		}

		for _, r := range responses {
			if r.CommandId == command_id {
				result = r
				return nil // Stop the polling loop
			}
		}

		return errors.New(fmt.Sprintf(
			"Response for remote command %d not received. Results so far: %v",
			command_id, responses))
	}, &testing.PollOptions{Timeout: 60 * time.Second}); err != nil {
		return nil, errors.Wrap(err, "failed to wait for the remote command response")
	}

	return &result, nil
}
