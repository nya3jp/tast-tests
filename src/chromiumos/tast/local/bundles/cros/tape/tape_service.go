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

	"chromiumos/tast/errors"
	"chromiumos/tast/services/cros/tape"
	"chromiumos/tast/testing"
)

type TapeService struct {
	s *testing.ServiceState
}

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			tape.RegisterTapeServiceServer(srv, &TapeService{
				s: s,
			})
		},
	})
}

// SaveGenericAccountInfoToFile writes a leased account to a location on the DUT.
func (t TapeService) SaveGenericAccountInfoToFile(ctx context.Context, req *tape.SaveGenericAccountInfoToFileRequest) (*empty.Empty, error) {
	jsonData, err := json.Marshal(&struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}{
		Username: req.Username,
		Password: req.Password,
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
func (t TapeService) RemoveGenericAccountInfo(ctx context.Context, req *tape.RemoveGenericAccountInfoRequest) (*empty.Empty, error) {
	if err := os.Remove(req.Path); err != nil {
		return nil, errors.Wrap(err, "failed to remove file")
	}

	return &empty.Empty{}, nil
}
