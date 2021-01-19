// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"github.com/golang/protobuf/proto"

	tmpb "chromiumos/system_api/tpm_manager_proto"
	"chromiumos/tast/errors"
)

// UnmarshalLocalData unmarshal d into tmpb.LocalData; also returns encountered error if any
func UnmarshalLocalData(d []byte) (*tmpb.LocalData, error) {
	var out tmpb.LocalData
	if err := proto.Unmarshal(d, &out); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal")
	}
	return &out, nil
}

// MarshalLocalData marshal d into []byte; also returns encountered error if any
func MarshalLocalData(d *tmpb.LocalData) ([]byte, error) {
	marshalled, err := proto.Marshal(d)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal")
	}
	return marshalled, nil
}
