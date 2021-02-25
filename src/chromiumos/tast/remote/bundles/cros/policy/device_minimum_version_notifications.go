// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"encoding/json"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/policyutil"
	"chromiumos/tast/rpc"
	ps "chromiumos/tast/services/cros/policy"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DeviceMinimumVersionNotifications,
		Desc: "Notifications of DeviceMinimumVersion policy when device has reached auto update expiration",
		Contacts: []string{
			"snijhara@google.com", // Test author
			"marcgrimme@google.com",
		},
		Attr:         []string{"group:enrollment"},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.policy.PolicyService"},
		Timeout:      7 * time.Minute,
	})
}

func DeviceMinimumVersionNotifications(ctx context.Context, s *testing.State) {
	defer func(ctx context.Context) {
		if err := policyutil.EnsureTPMAndSystemStateAreReset(ctx, s.DUT()); err != nil {
			s.Error("Failed to reset TPM: ", err)
		}
	}(ctx)

	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Minute)
	defer cancel()

	if err := policyutil.EnsureTPMAndSystemStateAreReset(ctx, s.DUT()); err != nil {
		s.Fatal("Failed to reset TPM: ", err)
	}

	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	pc := ps.NewPolicyServiceClient(cl.Conn)

	// Start with empty policy and set policy value after logging into the session so that
	// the user can be shown the notification.
	emptyPb := fakedms.NewPolicyBlob()
	emptyJSON, err := json.Marshal(emptyPb)
	if err != nil {
		s.Fatal("Failed to serialize empty policies: ", err)
	}

	if _, err := pc.EnrollUsingChrome(ctx, &ps.EnrollUsingChromeRequest{
		PolicyJson: emptyJSON,
		ExtraArgs:  "--aue-reached-for-update-required-test",
	}); err != nil {
		s.Fatal("Failed to enroll using chrome: ", err)
	}
	defer pc.StopChromeAndFakeDMS(ctx, &empty.Empty{})

	s.Log("Updating policies in session")
	minimumVersionPb := fakedms.NewPolicyBlob()
	minimumVersionPb.AddPolicy(&policy.DeviceMinimumVersion{
		Val: &policy.DeviceMinimumVersionValue{
			Requirements: []*policy.DeviceMinimumVersionValueRequirements{
				{
					AueWarningPeriod: 7,
					ChromeosVersion:  "99999999",
					WarningPeriod:    0,
				},
			},
		},
	})

	minimumVersionJSON, err := json.Marshal(minimumVersionPb)
	if err != nil {
		s.Fatal("Failed to serialize policies: ", err)
	}
	pc.UpdatePolicies(ctx, &ps.UpdatePoliciesRequest{
		PolicyJson: minimumVersionJSON,
	})

	// Notification Id is hardcoded in Chrome.
	if _, err = pc.VerifyVisibleNotification(ctx, &ps.VerifyVisibleNotificationRequest{
		NotificationId: "policy.update_required",
	}); err != nil {
		s.Error("Failed to verify DeviceMinimumVersion policy notification: ", err)
	}

	if _, err = pc.EvalExpressionInChromeURL(ctx, &ps.EvalExpressionInChromeUrlRequest{
		Url:        "chrome://management/",
		Expression: "document.querySelector('management-ui').$$('.eol-section') && !document.querySelector('management-ui').$$('.eol-section[hidden]')",
	}); err != nil {
		s.Error("Failed to verify device minimum version policy banner on management settings: ", err)
	}
}
