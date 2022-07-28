// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package tape

import (
	"context"
	"time"

	tape2 "chromiumos/tast/common/tape"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         GenericAccounts,
		Desc:         "Confirm that the generic account leasing for TAPE works as intended",
		LacrosStatus: testing.LacrosVariantUnneeded,
		Contacts:     []string{"davidwelling@google.com", "alexanderhartl@google.com", "arc-engprod@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      5 * time.Minute,
		Vars:         []string{tape2.LocalRefreshTokenVar},
		VarDeps:      []string{tape2.AuthenticationConfigJSONVar},
		// TODO(b/240322950): Remove the remote fixture requirement at some point.
		Fixture: "tapeRemoteBase",
		// Limit to eve. This test does not need to run on more than one device.
		HardwareDeps: hwdep.D(hwdep.Model("eve")),
	})
}

func GenericAccounts(ctx context.Context, s *testing.State) {
	// Ensure empty creds are set to ensure that service account in the lab is
	// used when running remotely, and that local credentials are passed in when running locally.
	// See the TAPE Readme for more information.
	var emptyCreds []byte
	client, err := tape2.NewClient(ctx, emptyCreds)
	if err != nil {
		s.Fatal("Failed to setup the TAPE client: ", err)
	}

	// Lease the account.
	account, err := client.RequestGenericAccount(ctx, tape2.WithPoolID("tape.generic_accounts_test"), tape2.WithTimeout(5*60))
	if err != nil {
		s.Fatal("Failed to request a generic account: ", err)
	}

	s.Logf("Received account with username: %s", account.Username)

	// Release the account.
	if err := client.ReleaseGenericAccount(ctx, account); err != nil {
		s.Fatal("Failed to release the generic account: ", err)
	}
}
