// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package autoupdate

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"golang.org/x/sys/unix"
	"google.golang.org/grpc"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	aupb "chromiumos/tast/services/cros/autoupdate"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			aupb.RegisterNebraskaServiceServer(srv, &NebraskaService{s: s})
		},
	})
}

// NebraskaService implements tast.cros.policy.NebraskaService.
type NebraskaService struct {
	s *testing.ServiceState

	cmd     *testexec.Cmd
	root    string
	logPath string
}

// CreateTempDir creates a temporary directory that is used by Nebraska.
func (nebraska *NebraskaService) CreateTempDir(ctx context.Context, req *empty.Empty) (*aupb.CreateTempDirResponse, error) {
	dir, err := ioutil.TempDir("", "nebraska")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create temp dir")
	}
	nebraska.root = dir

	return &aupb.CreateTempDirResponse{Path: dir}, nil
}

// Start starts a Nebraska service instance with the given parameters.
func (nebraska *NebraskaService) Start(ctx context.Context, req *aupb.StartRequest) (*aupb.StartResponse, error) {
	root := nebraska.root
	logPath := filepath.Join(root, "nebraska.log")

	// Collect the arguments.
	args := []string{
		"--runtime-root", root,
		"--log-file", logPath,
	}

	if req.Port != "" {
		testing.ContextLog(ctx, "Adding port to arguments")
		args = append(args, "--port", req.Port)
	}

	if req.Update != nil {
		testing.ContextLog(ctx, "Adding update to arguments")
		args = append(args,
			"--update-metadata", req.Update.MetadataFolder,
			"--update-payloads-address", req.Update.Address,
		)
	}

	// Start the Nebraska service.
	nebraska.cmd = testexec.CommandContext(nebraska.s.ServiceContext(), "nebraska.py", args...)

	testing.ContextLog(ctx, "Starting Nebraska")
	if err := nebraska.cmd.Start(); err != nil {
		return nil, errors.Wrap(err, "failed to start Nebraska service")
	}

	// Wait for the port file.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if _, err := os.Stat(filepath.Join(root, "port")); err != nil {
			if os.IsNotExist(err) {
				return err
			}
			return testing.PollBreak(err)
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second, Interval: time.Second}); err != nil {
		return nil, errors.Wrap(err, "failed to find the port file")
	}

	// Get and check the port number.
	port, err := ioutil.ReadFile(filepath.Join(root, "port"))
	if err != nil {
		return nil, errors.Wrap(err, "failed to read the Nebraska's port file")
	} else if req.Port != "" && req.Port != string(port) {
		return nil, errors.Errorf("Nebraska started with wrong port; want %s, got %s", req.Port, string(port))
	}

	return &aupb.StartResponse{
		Port:    string(port),
		LogPath: logPath,
	}, nil
}

// Stop gracefully stops the previously started Nebraska instance.
func (nebraska *NebraskaService) Stop(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if nebraska.cmd == nil {
		testing.ContextLog(ctx, "There is no Nebraska process to stop")
		return &empty.Empty{}, nil
	}

	testing.ContextLog(ctx, "Stopping Nebraska")

	if err := nebraska.cmd.Process.Signal(unix.SIGINT); err != nil {
		testing.ContextLog(ctx, "Failed to interrupt the Nebraska process: ", err)
		return nil, errors.Wrap(err, "failed to interrupt the Nebraska process")
	}

	ok := false
	errc := make(chan error)
	go func() {
		errc <- nebraska.cmd.Wait(testexec.DumpLogOnError)
	}()

	select {
	case err := <-errc:
		if err == nil {
			ok = true
		}
	case <-ctx.Done():
	case <-time.After(3 * time.Second):
	}
	if !ok {
		testing.ContextLog(ctx, "Failed to wait until the Nebraska process stopped")
		return nil, errors.New("failed to wait until the Nebraska process stopped")
	}

	return &empty.Empty{}, nil
}

// RemoveTempDir removes the temporary directory that was created for Nebraska.
func (nebraska *NebraskaService) RemoveTempDir(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	testing.ContextLog(ctx, "Deleting temp dir")
	if err := os.RemoveAll(nebraska.root); err != nil {
		testing.ContextLogf(ctx, "Failed to delete %s: %v", nebraska.root, err)
	}

	return &empty.Empty{}, nil
}
