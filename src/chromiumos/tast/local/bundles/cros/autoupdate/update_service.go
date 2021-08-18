// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package autoupdate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/lsbrelease"
	aupb "chromiumos/tast/services/cros/autoupdate"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			aupb.RegisterUpdateServiceServer(srv, &UpdateService{s: s})
		},
	})
}

// UpdateService implements tast.cros.autoupdate.UpdateService.
type UpdateService struct { // NOLINT
	s *testing.ServiceState
}

const statefulPath = "/mnt/stateful_partition/etc/lsb-release"

// CheckForUpdate starts update_engine_client to update the OS.
func (u *UpdateService) CheckForUpdate(ctx context.Context, req *aupb.UpdateRequest) (*empty.Empty, error) {
	// Collect the arguments.
	args := []string{"--update"}

	if req.OmahaUrl != "" {
		testing.ContextLog(ctx, "Adding Omaha URL to arguments")
		args = append(args, fmt.Sprintf("--omaha_url=%s", req.OmahaUrl))
	}

	cmd := testexec.CommandContext(ctx, "update_engine_client", args...)

	testing.ContextLog(ctx, "Starting the update")
	_, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to start update engine client")
	}

	return &empty.Empty{}, nil
}

// LSBReleaseContent gets the content of /etc/lsb-release.
func (u *UpdateService) LSBReleaseContent(ctx context.Context, req *empty.Empty) (*aupb.LSBRelease, error) {
	content, err := lsbrelease.Load()
	if err != nil {
		return &aupb.LSBRelease{}, errors.Wrap(err, "failed to load lsbrelease information from /etc/lsb-release")
	}

	contentJSON, err := json.Marshal(content)
	if err != nil {
		return &aupb.LSBRelease{}, errors.Wrap(err, "failed to serialize the content of /etc/lsb-release")
	}

	return &aupb.LSBRelease{ContentJson: contentJSON}, nil
}

// StatefulLSBReleaseContent gets the content of /mnt/stateful_partition/etc/lsb-release.
// The values in this file overwrite the effect of the ones in /etc/lsb-release.
func (u *UpdateService) StatefulLSBReleaseContent(ctx context.Context, req *empty.Empty) (*aupb.LSBRelease, error) {
	content, err := lsbrelease.LoadFrom(statefulPath)
	if err != nil {
		return &aupb.LSBRelease{}, errors.Wrapf(err, "failed to retreive lsbrelease information from %s", statefulPath)
	}

	contentJSON, err := json.Marshal(content)
	if err != nil {
		return &aupb.LSBRelease{}, errors.Wrapf(err, "failed to serialize the content of %s", statefulPath)
	}

	return &aupb.LSBRelease{ContentJson: contentJSON}, nil
}

// OverwriteStatefulLSBRelease overwrites the content of /mnt/stateful_partition/etc/lsb-release.
func (u *UpdateService) OverwriteStatefulLSBRelease(ctx context.Context, req *aupb.LSBRelease) (*empty.Empty, error) {
	var content map[string]string
	if err := json.Unmarshal(req.ContentJson, &content); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal the request content")
	}

	output := new(bytes.Buffer)
	for key, value := range content {
		fmt.Fprintf(output, "%s=%s\n", key, value)
	}

	err := ioutil.WriteFile(statefulPath, output.Bytes(), 0644)
	if err != nil {
		return &empty.Empty{}, errors.Wrapf(err, "failed to write the new content to %s", statefulPath)
	}

	return &empty.Empty{}, nil
}
