// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package webrtcinternals

import (
	"fmt"
	"strings"

	"chromiumos/tast/errors"
)

// StatsKey indexes the Stats field of PeerConnection.
type StatsKey struct {
	ID        string
	Attribute string
}

// MarshalText encodes StatsKey.
func (key StatsKey) MarshalText() ([]byte, error) {
	return []byte(fmt.Sprintf("%s-%s", key.ID, key.Attribute)), nil
}

// UnmarshalText decodes StatsKey.
func (key *StatsKey) UnmarshalText(text []byte) error {
	var found bool
	key.ID, key.Attribute, found = strings.Cut(string(text), "-")
	if !found {
		return errors.Errorf("dash not found in %q", text)
	}
	return nil
}
