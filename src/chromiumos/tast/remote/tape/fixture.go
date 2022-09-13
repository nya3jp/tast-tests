// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package tape holds remote features required to use TAPE.
package tape

import (
	"time"

	"chromiumos/tast/common/tape"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:            "tapeRemoteBase",
		Desc:            "Writes a TAPE access token to the DUT in a shared location for accessing the TAPE API from the DUT",
		Contacts:        []string{"davidwelling@google.com", "arc-engprod@google.com"},
		Impl:            tape.NewBaseTapeFixture(),
		Vars:            []string{tape.AuthenticationConfigJSONVar, tape.LocalRefreshTokenVar},
		SetUpTimeout:    10 * time.Second,
		TearDownTimeout: 10 * time.Second,
		PreTestTimeout:  1 * time.Minute,
		PostTestTimeout: 1 * time.Minute,
	})

}
