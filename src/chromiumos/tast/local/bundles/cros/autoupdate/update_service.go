// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package autoupdate

import (
	"context"
	"fmt"
	"io/ioutil"
	"regexp"
	"strings"

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

func (Update *UpdateService) CheckForUpdate(ctx context.Context, req *aupb.UpdateRequest) (*empty.Empty, error) {
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

func (Update *UpdateService) LSBReleaseContent(ctx context.Context, req *empty.Empty) (*aupb.LSBRelease, error) {
	lsb, err := lsbrelease.Load()
	if err != nil {
		return &aupb.LSBRelease{}, errors.Wrap(err, "unable to retreive lsbrelease information")
	}

	return &aupb.LSBRelease{
		Board:         lsb[lsbrelease.Board],
		BuilderPath:   lsb[lsbrelease.BuilderPath],
		Milestone:     lsb[lsbrelease.Milestone],
		BuildNumber:   lsb[lsbrelease.BuildNumber],
		PatchNumber:   lsb[lsbrelease.PatchNumber],
		Version:       lsb[lsbrelease.Version],
		ReleaseAppId:  lsb[lsbrelease.ReleaseAppID],
		BuildType:     lsb[lsbrelease.BuildType],
		ReleaseTrack:  lsb[lsbrelease.ReleaseTrack],
		ArcSdkVersion: lsb[lsbrelease.ARCSDKVersion],
		ArcVersion:    lsb[lsbrelease.ARCVersion],
	}, nil
}

func (Update *UpdateService) LSBReleaseOverwriteContent(ctx context.Context, req *empty.Empty) (*aupb.LSBRelease, error) {
	lsb, err := lsbrelease.LoadFrom(statefulPath)
	if err != nil {
		return &aupb.LSBRelease{}, errors.Wrap(err, "unable to retreive lsbrelease information")
	}

	lsbResponse := &aupb.LSBRelease{}
	if board, ok := lsb[lsbrelease.Board]; ok {
		lsbResponse.Board = board
	}
	if track, ok := lsb[lsbrelease.ReleaseTrack]; ok {
		lsbResponse.ReleaseTrack = track
	}

	return lsbResponse, nil
}

func (Update *UpdateService) OverwriteLSBRelease(ctx context.Context, req *aupb.LSBRelease) (*empty.Empty, error) {
	overwrites := make(map[string]string)

	if req.Board != "" {
		overwrites[lsbrelease.Board] = req.Board
	}

	input, err := ioutil.ReadFile(statefulPath)
	if err != nil {
		return &empty.Empty{}, err
	}

	lineRe := regexp.MustCompile(`^([A-Z0-9_]+)\s*=\s*(.*)$`)

	// Delete the content of the lines we want to overwrite.
	lines := strings.Split(string(input), "\n")
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		m := lineRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}

		// Empty the line if it has a key we want to overwrite.
		if _, ok := overwrites[m[1]]; ok {
			lines[i] = ""
		}
	}

	// Append the new values.
	for key, val := range overwrites {
		lines = append(lines, fmt.Sprintf("%s=%s", key, val))
	}

	// Remove the empty lines.
	output := regexp.MustCompile(`[\t\r\n]+`).ReplaceAllString(strings.Join(lines, "\n"), "\n")
	err = ioutil.WriteFile(statefulPath, []byte(output+"\n"), 0644)
	if err != nil {
		return &empty.Empty{}, err
	}

	return &empty.Empty{}, nil
}
