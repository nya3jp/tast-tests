// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package tape

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
)

// ServiceAccountVar holds the name of the variable which stores the service account credentials for TAPE.
const ServiceAccountVar = "tape.service_account_key"

// AuthenticationConfigJSONVar holds the name of the variable which stores
// the authentication config variables for TAPE.
const AuthenticationConfigJSONVar = "tape.authentication_config_json"

// LocalRefreshTokenVar holds the name of the variable which stores the
// local refresh token for TAPE authentication.
const LocalRefreshTokenVar = "tape.local_refresh_token"

// dutServiceAccountDir holds the path that service accounts from the host
// will be stored on the DUT.
const dutServiceAccountDir = "/tmp/tape/service_account"

// AuthenticationConfig represents the data that's stored in the AuthenticationConfigJSONVar.
type AuthenticationConfig struct {
	ClientID                     string   `json:"client_id"`
	ClientSecret                 string   `json:"client_secret"`
	DroneServiceAccountLocations []string `json:"drone_service_account_locations"`
}

// serviceAccountPathOnDUT returns the path that a service account should be saved
// to on the DUT given the name from the remote host.
func serviceAccountPathOnDUT(pathOnHost string) string {
	return fmt.Sprintf("%s/%s", dutServiceAccountDir, filepath.Base(pathOnHost))
}

// LeaseAccount leases an account from the given poolID for the provided amount of time and optionally locks it.
func LeaseAccount(ctx context.Context, poolID string, leaseLength time.Duration, lockAccount bool, credsJSON []byte) (account *Account, cleanup func(ctx context.Context) error, err error) {
	client, err := NewTapeClient(ctx, WithCredsJSON(credsJSON))
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to setup TAPE client")
	}

	params := NewRequestOTAParams(int32(leaseLength.Seconds()), &poolID, false)
	acc, err := RequestAccount(ctx, client, params)
	if err != nil {
		return acc, nil, errors.Wrap(err, "failed to request an account")
	}

	return acc, func(ctx context.Context) error {
		if err := acc.ReleaseAccount(ctx, client); err != nil {
			return errors.Wrap(err, "failed to release an account")
		}
		return nil
	}, nil
}

// LeaseGenericAccount leases a generic account for the given pool.
func LeaseGenericAccount(ctx context.Context, poolID string, leaseLength time.Duration, localRefreshToken string, authenticationConfigJSON []byte) (account *GenericAccount, cleanup func(ctx context.Context) error, err error) {
	client, err := NewTapeClient(ctx, WithLocalAndRemoteAuthentication(localRefreshToken, authenticationConfigJSON))
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to setup TAPE client")
	}

	params := RequestGenericAccountParams{
		TimeoutInSeconds: int32(leaseLength.Seconds()),
		PoolID:           &poolID,
	}

	gar, err := RequestGenericAccount(ctx, params, client)
	if err != nil {
		return gar, nil, errors.Wrap(err, "failed to request generic account")
	}

	return gar, func(ctx context.Context) error {
		if err := ReleaseGenericAccount(ctx, gar, client); err != nil {
			return errors.Wrap(err, "failed to release generic account")
		}

		return nil
	}, nil
}

// NewBaseTapeFixture creates a base fixture that loads service accounts needed
// by tape from the remote host to the DUT. This fixture must be instantiated
// and extended by any test that uses TAPE.
func NewBaseTapeFixture() *tapeBaseFixt {
	return &tapeBaseFixt{}
}

type tapeBaseFixt struct{}

func (*tapeBaseFixt) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	// TODO(b/204845193): workaround as fixture do not recover connection
	if err := s.DUT().Connect(ctx); err != nil {
		s.Fatal("Failed to reconnect to DUT: ", err)
	}

	// Parse out the shared variables.
	sharedVars, ok := s.Var(AuthenticationConfigJSONVar)
	if !ok {
		s.Fatal("Authentication config variable not set")
	}

	var parsedSharedVars AuthenticationConfig
	if err := json.Unmarshal([]byte(sharedVars), &parsedSharedVars); err != nil {
		s.Fatal("Failed to unmarshal data: ", err)
	}

	// Clear out the path to where the files will live on the DUT.
	if _, err := s.DUT().Conn().CommandContext(ctx, "rm", "-rf", dutServiceAccountDir).Output(); err != nil {
		s.Fatal("Failed to remove credentials directory: ", err)
	}

	// Write each of the files from the server down to the host.
	var credentialPaths = make(map[string]string)
	for _, path := range parsedSharedVars.DroneServiceAccountLocations {
		// Make sure the path exists.
		if _, err := os.Stat(path); err != nil {
			s.Logf("service account does not exist on host at: %s", path)
			continue
		}

		credentialPaths[path] = serviceAccountPathOnDUT(path)
	}

	if _, err := linuxssh.PutFiles(ctx, s.DUT().Conn(), credentialPaths, linuxssh.PreserveSymlinks); err != nil {
		s.Fatal("Failed to write credentials to DUT: ", err)
	}

	return nil
}
func (*tapeBaseFixt) TearDown(ctx context.Context, s *testing.FixtState) {
	// Remove the credentials directory.
	if _, err := s.DUT().Conn().CommandContext(ctx, "rm", "-rf", dutServiceAccountDir).Output(); err != nil {
		s.Fatal("Failed to remove credentials directory: ", err)
	}
}
func (*tapeBaseFixt) Reset(ctx context.Context) error                        { return nil }
func (*tapeBaseFixt) PreTest(ctx context.Context, s *testing.FixtTestState)  {}
func (*tapeBaseFixt) PostTest(ctx context.Context, s *testing.FixtTestState) {}
