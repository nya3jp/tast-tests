// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	pb "chromiumos/tast/services/cros/policy"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			pb.RegisterRequiredClientCertificateForDeviceServiceServer(srv, &RequiredClientCertificateForDeviceService{s: s})
		},
	})
}

// RequiredClientCertificateForDeviceService implements tast.cros.policy.RequiredClientCertificateForDeviceService.
type RequiredClientCertificateForDeviceService struct { // NOLINT
	s *testing.ServiceState
}

// TestClientCertificateIsInstalled uses the command line to poll for all installed certificates and returns when
// the correct one is found without an error or returns with an error if the certificate wasn't found after 60s.
func (c *RequiredClientCertificateForDeviceService) TestClientCertificateIsInstalled(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {

	// Wait until the certificate is installed.
	if err := testing.Poll(ctx, func(ctx context.Context) error {

		out, err := testexec.CommandContext(ctx, "pkcs11-tool", "--module", "/usr/lib64/libchaps.so", "--slot", "0", "--list-objects").Output()
		if err != nil {
			return errors.Wrap(err, "failed to get certificate list")
		}
		outStr := strings.TrimSpace(string(out))

		if !strings.Contains(outStr, "TastTest") {
			return errors.New("certificate not installed")
		}

		return nil

	}, &testing.PollOptions{
		Timeout: 60 * time.Second,
	}); err != nil {
		return nil, err
	}

	return &empty.Empty{}, nil
}
