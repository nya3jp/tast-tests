// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package webrtcinternals

import "fmt"

// PeerConnectionID indexes the PeerConnections field of Dump.
type PeerConnectionID struct {
	Rid int
	Lid int
}

// peerConnectionIDFmt is a string format specifier for PeerConnectionID.
const peerConnectionIDFmt = "%d-%d"

// MarshalText encodes PeerConnectionID with peerConnectionIDFmt.
func (id PeerConnectionID) MarshalText() ([]byte, error) {
	return []byte(fmt.Sprintf(peerConnectionIDFmt, id.Rid, id.Lid)), nil
}

// UnmarshalText decodes PeerConnectionID with peerConnectionIDFmt.
func (id *PeerConnectionID) UnmarshalText(text []byte) error {
	_, err := fmt.Sscanf(string(text), peerConnectionIDFmt, &id.Rid, &id.Lid)
	return err
}
