// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package tape

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	tape2 "chromiumos/tast/common/tape"
	"chromiumos/tast/errors"
	"chromiumos/tast/services/cros/tape"
	"chromiumos/tast/testing"
)

// LeasedAccountManagementService manages a leased account and makes it
// available to a local DUT by writing to the DUTs filesystem.
type LeasedAccountManagementService struct {
	s *testing.ServiceState
}

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			tape.RegisterTapeServiceServer(srv, &LeasedAccountManagementService{
				s: s,
			})
		},
	})
}

// SaveGenericAccountInfoToFile writes a leased account to a location on the DUT.
func (t LeasedAccountManagementService) SaveGenericAccountInfoToFile(ctx context.Context, req *tape.SaveGenericAccountInfoToFileRequest) (*empty.Empty, error) {
	jsonData, err := json.Marshal(tape2.LeasedAccountFileData{
		Username: req.Username,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal data")
	}

	if err := ioutil.WriteFile(req.Path, jsonData, 0644); err != nil {
		return nil, errors.Wrap(err, "failed to write file")
	}

	return &empty.Empty{}, nil
}

// RemoveGenericAccountInfo removes a leased account from the DUT.
func (t LeasedAccountManagementService) RemoveGenericAccountInfo(ctx context.Context, req *tape.RemoveGenericAccountInfoRequest) (*empty.Empty, error) {
	if err := os.Remove(req.Path); err != nil {
		return nil, errors.Wrap(err, "failed to remove file")
	}

	return &empty.Empty{}, nil
}
