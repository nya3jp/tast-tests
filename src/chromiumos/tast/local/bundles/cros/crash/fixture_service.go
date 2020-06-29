// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package crash contains RPC wrappers to set up and tear down tests.
package crash

import (
	"os"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/crash"
	crash_service "chromiumos/tast/services/cros/crash"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			crash_service.RegisterFixtureServiceServer(srv, &FixtureService{s: s})
		},
	})
}

// FixtureService implements tast.cros.crash.FixtureService
type FixtureService struct {
	s *testing.ServiceState

	cr *chrome.Chrome
}

func (c *FixtureService) SetUp(ctx context.Context, req *crash_service.SetUpCrashTestRequest) (*empty.Empty, error) {
	consentOpt := crash.WithMockConsent()
	if req.Consent == crash_service.SetUpCrashTestRequest_REAL_CONSENT {
		if c.cr != nil {
			testing.ContextLog(ctx, "Already set up. Not setting up again")
			return nil, errors.New("already set up")
		}

		cr, err := chrome.New(ctx, chrome.ExtraArgs(crash.ChromeVerboseConsentFlags))
		if err != nil {
			testing.ContextLog(ctx, "Error setting up chrome: ", err)
			return nil, err
		}
		c.cr = cr
		consentOpt = crash.WithConsent(cr)
	}

	if err := crash.SetUpCrashTest(ctx, consentOpt, crash.RebootingTest()); err != nil {
		testing.ContextLog(ctx, "Error setting up crash test: ", err)
		return nil, err
	}
	return &empty.Empty{}, nil
}

func (c *FixtureService) EnableCrashFilter(ctx context.Context, req *crash_service.EnableCrashFilterRequest) (*empty.Empty, error) {
	return &empty.Empty{}, crash.EnableCrashFiltering(ctx, req.Name)
}

func (c *FixtureService) DisableCrashFilter(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	return &empty.Empty{}, crash.DisableCrashFiltering()
}

func (c *FixtureService) WaitForCrashFiles(ctx context.Context, req *crash_service.WaitForCrashFilesRequest) (*crash_service.WaitForCrashFilesResponse, error) {
	if len(req.GetDirs()) == 0 {
		testing.ContextLog(ctx, "Need to specify directories to examine")
		return nil, errors.New("need to specify directories to examine")
	}
	if len(req.GetRegexes()) == 0 {
		testing.ContextLog(ctx, "Need to specify regexes to search for")
		return nil, errors.New("need to specify regexes to search for")
	}

	// The reboot tests generally rely on boot collection being done, so
	// wait for that.
	// Boot collection can take a while to start, so use a long timeout.
	if err := testing.Poll(ctx, func(c context.Context) error {
		if _, err := os.Stat("/run/crash_reporter/boot-collector-done"); err != nil {
			if os.IsNotExist(err) {
				return err
			}
			return testing.PollBreak(errors.Wrap(err, "failed to check boot-collector-done"))
		}
		return nil
	}, &testing.PollOptions{Timeout: 120 * time.Second}); err != nil {
		return nil, errors.Wrap(err, "boot_collector did not complete")
	}

	files, err := crash.WaitForCrashFiles(ctx, req.GetDirs(), []string(nil), req.GetRegexes())
	if err != nil {
		testing.ContextLog(ctx, "Failed to wait for crash files: ", err)
		return nil, errors.Wrap(err, "failed to wait for crash files")
	}
	out := crash_service.WaitForCrashFilesResponse{}
	for k, v := range files {
		out.Matches = append(out.Matches, &crash_service.RegexMatch{
			Regex: k,
			Files: v,
		})
	}
	return &out, nil
}

func (c *FixtureService) RemoveAllFiles(ctx context.Context, req *crash_service.RemoveAllFilesRequest) (*empty.Empty, error) {
	files := make(map[string][]string)
	for _, m := range req.Matches {
		files[m.Regex] = m.Files
	}
	return &empty.Empty{}, crash.RemoveAllFiles(ctx, files)
}

func (c *FixtureService) TearDown(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	var firstErr error
	if err := crash.TearDownCrashTest(ctx); err != nil {
		testing.ContextLog(ctx, "Error tearing down: ", err)
		firstErr = errors.Wrap(err, "error tearing down fixture: ")
	}
	if c.cr != nil {
		// c.cr could be nil if the machine rebooted in the middle,
		// so don't complain if it is.
		if err := c.cr.Close(ctx); err != nil {
			testing.ContextLog(ctx, "Error closing Chrome: ", err)
			if firstErr == nil {
				firstErr = errors.Wrap(err, "error closing Chrome:")
			}
		}
		c.cr = nil
	}
	return &empty.Empty{}, firstErr
}
