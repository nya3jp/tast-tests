// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package enterprise

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/enterprise/arcent"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash/ashproc"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/session"
	pb "chromiumos/tast/services/cros/enterprise"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			pb.RegisterArcSnapshotServiceServer(srv, &ArcSnapshotService{})
		},
	})
}

// ArcSnapshotService implements tast.cros.enterprise.ArcSnapshotService.
type ArcSnapshotService struct {
	cr *chrome.Chrome
}

// Enroll the device with the provided account credentials.
func (service *ArcSnapshotService) Enroll(ctx context.Context, req *pb.EnrollRequest) (_ *empty.Empty, retErr error) {
	if service.cr != nil {
		return nil, errors.New("DUT for running snapshot is already set up")
	}
	var opts []chrome.Option

	opts = append(opts, chrome.GAIAEnterpriseEnroll(chrome.Creds{User: req.User, Pass: req.Pass}))
	opts = append(opts, chrome.ARCSupported())
	opts = append(opts, chrome.ProdPolicy())
	opts = append(opts, chrome.NoLogin())
	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to Chrome")
	}

	service.cr = cr
	return &empty.Empty{}, nil
}

// WaitForPackagesInMgs waits for the list of packages being installed in MGS
// and returns the perf data.
// If service.cr is not null (the first call after Enroll()), the related user
// policy values are checked and service.cr is disconnected.
func (service *ArcSnapshotService) WaitForPackagesInMgs(origCtx context.Context, req *pb.WaitForPackagesInMgsRequest) (res *pb.WaitForPackagesInMgsResponse, retErr error) {
	name := req.Name
	user := req.User
	isHeadless := req.IsHeadless
	packages := req.Packages

	perfValues := perf.NewValues()

	ctx, cancel := context.WithTimeout(origCtx, 10*time.Minute)
	defer cancel()

	outDir, ok := testing.ContextOutDir(ctx)
	if !ok {
		return nil, errors.New("failed to get outDir")
	}

	if user != "" {
		if err := waitForCryptohome(ctx, user); err != nil {
			return nil, err
		}
		testing.ContextLog(ctx, "Mounted user system path")
	}

	startTime := time.Now()

	// Ensure that ARC is launched.
	a, err := arc.New(ctx, outDir)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start ARC by user policy")
	}
	defer a.Close(ctx)

	if user == "" {
		primaryUser, err := getPrimaryUser(ctx)
		if err != nil {
			return nil, err
		}
		user = primaryUser
		if err := waitForCryptohome(ctx, user); err != nil {
			return nil, err
		}
	}

	isHeadlessPlatform, err := isHeadlessPlatform()
	if err != nil {
		return nil, err
	}
	if isHeadlessPlatform != isHeadless {
		if isHeadless {
			return nil, errors.New("Chrome is expected to be running headless")
		}
		return nil, errors.New("Chrome is expected to be running with UI, but running headless")
	}
	if service.cr != nil {
		tconn, err := service.cr.TestAPIConn(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "failed to connect to test api")
		}
		// Ensure chrome://policy shows correct ArcEnabled value.
		if err := policyutil.Verify(ctx, tconn, []policy.Policy{&policy.ArcEnabled{Val: true}}); err != nil {
			return nil, errors.Wrap(err, "failed to verify ArcEnabled")
		}

		interval := policy.RefWeeklyTimeIntervals{
			End: &policy.RefWeeklyTime{
				DayOfWeek: "SATURDAY",
				Time:      0,
			},
			Start: &policy.RefWeeklyTime{
				DayOfWeek: "MONDAY",
				Time:      0,
			}}
		intervals := []*policy.RefWeeklyTimeIntervals{&interval}

		// Ensure chrome://policy shows correct DeviceArcDataSnapshotHours value.
		if err := policyutil.Verify(ctx, tconn, []policy.Policy{&policy.DeviceArcDataSnapshotHours{Val: &policy.DeviceArcDataSnapshotHoursValue{
			Intervals: intervals}}}); err != nil {
			return nil, errors.Wrap(err, "failed to verify DeviceArcDataSnapshotHours")
		}

		if err := arcent.VerifyArcPolicyForceInstalled(ctx, tconn, packages); err != nil {
			return nil, errors.Wrap(err, "failed to verify force-installed apps in ArcPolicy")
		}
	}

	// Ensure that Android packages are force-installed by ARC policy.
	// Note: if the user policy for the user is changed, the packages listed in
	// credentials files must be updated.
	if err := a.WaitForPackages(ctx, packages); err != nil {
		return nil, errors.Wrap(err, "failed to force install packages")
	}
	duration := time.Since(startTime)

	perfValues.Set(perf.Metric{
		Name:      name,
		Unit:      "seconds",
		Direction: perf.SmallerIsBetter,
	}, duration.Seconds())

	testing.ContextLog(ctx, "Time: ", duration.Round(time.Millisecond))

	if service.cr != nil {
		defer service.cr.Close(ctx)
		service.cr = nil
	}
	return &pb.WaitForPackagesInMgsResponse{User: user, Perf: perfValues.Proto()}, nil
}

// WaitForSnapshot waits for snapshot folders being created.
func (service *ArcSnapshotService) WaitForSnapshot(ctx context.Context, req *pb.WaitForSnapshotRequest) (_ *empty.Empty, retErr error) {
	snapshots := req.SnapshotNames
	for _, snapshot := range snapshots {
		if snapshot != "last" && snapshot != "previous" {
			return nil, errors.Errorf("invalid snapshot name: %s", snapshot)
		}
		testing.ContextLog(ctx, "Wait for /var/cache/arc-data-snapshot/", snapshot)
		testing.Poll(ctx, func(ctx context.Context) error {
			m, err := filepath.Glob("/var/cache/arc-data-snapshot/" + snapshot)
			if err != nil {
				return errors.Wrap(err, "Snapshot path does not exist yet")
			}
			if len(m) == 0 {
				return errors.New("Snapshot path does not exist yet")
			}
			return nil
		}, &testing.PollOptions{Interval: time.Second})
	}
	return &empty.Empty{}, nil
}

// waitForCryptohome waits for a system path for the user is mounted.
func waitForCryptohome(ctx context.Context, user string) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		systempath, err := cryptohome.SystemPath(ctx, user)
		if err != nil {
			return errors.Wrap(err, "failed to get the cryptohome directory for user")
		}
		if _, err := os.Open(systempath); os.IsNotExist(err) {
			return errors.Wrap(err, "System path for user does not exist")
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Minute, Interval: 3 * time.Second})
}

// getPrimaryUser returns a user name of the primary user.
func getPrimaryUser(ctx context.Context) (string, error) {
	sessionManager, err := session.NewSessionManager(ctx)
	if err != nil {
		return "", errors.Wrap(err, "couldn't start session manager")
	}

	user, _, err := sessionManager.RetrievePrimarySession(ctx)
	if err != nil {
		return "", errors.Wrap(err, "couldn't retrieve active sessions")
	}
	return user, nil
}

// isHeadlessPlatform returns true if Chrome browser is running with headless
// ozone platform.
func isHeadlessPlatform() (bool, error) {
	proc, err := ashproc.Root()
	if err != nil {
		return false, errors.Wrap(err, "failed to get root")
	}
	args, err := proc.CmdlineSlice()
	if err != nil {
		return false, errors.Wrap(err, "failed to get command line")
	}
	for _, a := range args {
		if a == "--ozone-platform=headless" {
			return true, nil
		}
	}
	return false, nil
}
