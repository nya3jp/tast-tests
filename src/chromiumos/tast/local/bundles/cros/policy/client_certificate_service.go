// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"strings"

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
			pb.RegisterClientCertificateServiceServer(srv, &ClientCertificateService{s: s})
		},
	})
}

// ClientCertificateService implements tast.cros.policy.ClientCertificateService.
type ClientCertificateService struct { // NOLINT
	s *testing.ServiceState
}

// TestClientCertificateIsInstalled uses the command line to poll for all
// installed certificates and returns when the correct one is found without an
// error or returns with an error if the certificate wasn't found after 60s.
func (c *ClientCertificateService) TestClientCertificateIsInstalled(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {

	// Wait until the certificate is installed.
	if err := testing.Poll(ctx, func(ctx context.Context) error {

		// The argument --slot 0 is for the system slot which will have
		// ID 0 as it is loaded first
		out, err := testexec.CommandContext(ctx, "pkcs11-tool", "--module", "/usr/lib64/libchaps.so", "--slot", "0", "--list-objects").Output()
		if err != nil {
			return errors.Wrap(err, "failed to get certificate list")
		}
		outStr := string(out)

		// Currently the policy_testserver.py serves a hardcoded certificate,
		// with TastTest as the issuer. So we look for that string to
		// identify the correct certificate.
		if !strings.Contains(outStr, "TastTest") {
			return errors.New("certificate not installed")
		}

		return nil

	}, nil); err != nil {
		return nil, err
	}

	return &empty.Empty{}, nil
}
