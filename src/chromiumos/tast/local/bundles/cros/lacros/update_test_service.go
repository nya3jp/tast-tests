// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacros

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"google.golang.org/grpc"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/lacros/launcher"
	proto "chromiumos/tast/services/cros/lacros"
	"chromiumos/tast/testing"
)

const (
	lacrosRootComponentPath = "/home/chronos/cros-components"
	corruptStatefulFilePath = "/mnt/stateful_partition/.corrupt_stateful"
	versionURL              = "chrome://version/"
)

var versionRegexp = regexp.MustCompile(`(\d+\.)(\d+\.)(\d+\.)(\d+)`)

type lacrosMetadata struct {
	Content struct {
		Version string `json:"version"`
	} `json:"content"`
}

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			proto.RegisterUpdateTestServiceServer(srv, &UpdateTestService{s: s})
		},
	})
}

// UpdateTestService implements tast.cros.lacros.UpdateTestService.
type UpdateTestService struct { // NOLINT
	s *testing.ServiceState
}

// VerifyUpdate checks if the expected version of Lacros is loaded successfully without crash given the browser contexts.
func (uts *UpdateTestService) VerifyUpdate(ctx context.Context, req *proto.VerifyUpdateRequest) (*proto.VerifyUpdateResponse, error) {
	// Setup Ash Chrome with the given context including options and login info.
	var opts []chrome.Option
	for _, opt := range req.AshContext.GetOpts() {
		opts = append(opts, chrome.Option(chrome.ExtraArgs(opt)))
	}
	cr, err := uts.setupAshChrome(ctx, opts)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to Ash Chrome")
	}
	defer func(ctx context.Context) error {
		if err := cr.Close(ctx); err != nil {
			return errors.Wrap(err, "failed to close Ash Chrome")
		}
		return nil
	}(ctx)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create test API connection for Ash Chrome")
	}
	defer tconn.Close()

	// Open Lacros.
	expectedVersionedLacrosDir := filepath.Join("/run/imageloader", req.ExpectedComponent, req.ExpectedVersion)
	expectedVersionedLacrosPath := filepath.Join(expectedVersionedLacrosDir, "chrome")
	l, err := lacros.LaunchFromShelf(ctx, tconn, expectedVersionedLacrosDir)
	if err != nil {
		return nil, errors.Wrap(err, "failed to launch Lacros browser from Shelf")
	}
	defer l.Close(ctx)

	// Verify Lacros updates.
	testing.ContextLogf(ctx, "Verifying provisioned Lacros at %v with UI: %v", expectedVersionedLacrosPath, req.UseUi)
	status := proto.TestResult_NO_STATUS
	statusDetails := ""
	if req.UseUi {
		// Verify on the chrome://version UI that it has the expected version in "Executable Path".
		// Note that "Google Chrome" on the page cannot be used for verification since it is read from the binary
		// and does not reflect the version that is mocked for test in runtime.
		const exprExecutablePath = `document.querySelector('#executable_path').innerText`
		actualVersionedLacrosPath, err := uts.getVersionInfoFromUI(ctx, l, exprExecutablePath)
		if err != nil {
			return nil, errors.Wrap(err, "failed to read Lacros executable path")
		}
		if actualVersionedLacrosPath == expectedVersionedLacrosPath {
			status = proto.TestResult_PASSED
			statusDetails = fmt.Sprintf("executable path: %v", actualVersionedLacrosPath)
		} else {
			status = proto.TestResult_FAILED
			statusDetails = fmt.Sprintf("executable path expected: %v, actual: %v", expectedVersionedLacrosPath, actualVersionedLacrosPath)
		}
	} else {
		// Verify without UI that the lacros process is running from the expected versioned path.
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			pids, err := launcher.PidsFromPath(ctx, expectedVersionedLacrosPath)
			if err != nil || len(pids) == 0 {
				return err
			}
			return nil
		}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
			return nil, errors.Wrap(err, "could not find running process")
		}
		status = proto.TestResult_PASSED
	}

	return &proto.VerifyUpdateResponse{
		Result: &proto.TestResult{Status: status, StatusDetails: statusDetails},
	}, nil
}

// ClearUpdate removes provisioned Lacros in the install path.
func (uts *UpdateTestService) ClearUpdate(ctx context.Context, req *proto.ClearUpdateRequest) (*proto.ClearUpdateResponse, error) {
	testing.ContextLog(ctx, "Clearing provisioned Lacros")
	// Unmount and remove mount points.
	matches, _ := filepath.Glob("/run/imageloader/lacros*/*")
	for _, match := range matches {
		if err := testexec.CommandContext(ctx, "umount", "-f", match).Run(); err != nil {
			testing.ContextLog(ctx, "Failed to unmount ", match)
		}
		if err := testexec.CommandContext(ctx, "rm", "-rf", match).Run(); err != nil {
			testing.ContextLog(ctx, "Failed to remove ", match)
		}
	}

	// Remove provisioned files. Note that 'sh' is used to handle the glob.
	lacrosComponentPathGlob := filepath.Join(lacrosRootComponentPath, "*")
	if err := testexec.CommandContext(ctx, "sh", "-c",
		strings.Join([]string{"rm", "-rf", lacrosComponentPathGlob}, " ")).Run(); err != nil {
		testing.ContextLog(ctx, "Failed to remove provisioned components at ", lacrosComponentPathGlob)
	}
	// If failed to remove, fall back to marking the stateful partition corrupt, so the provision can restore it.
	matches, err := filepath.Glob(lacrosComponentPathGlob)
	if err != nil || len(matches) > 0 {
		testing.ContextLog(ctx, "Mark stateful partition corrupt for the provision to restore it")
		testexec.CommandContext(ctx, "touch", corruptStatefulFilePath).Run()
	}
	return &proto.ClearUpdateResponse{}, nil
}

// GetBrowserVersion returns version info of the given browser type.
// If multiple Lacros browsers are provisioned in the stateful partition, all the versions will be returned.
func (uts *UpdateTestService) GetBrowserVersion(ctx context.Context, req *proto.GetBrowserVersionRequest) (*proto.GetBrowserVersionResponse, error) {
	var versions []string
	switch req.Browser {
	case proto.BrowserType_ASH:
		version, err := uts.getAshVersion(ctx)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get the version for %v", req.Browser)
		}
		versions = append(versions, version)

	case proto.BrowserType_LACROS_ROOTFS:
		version, err := uts.getLacrosRootfsVersion(ctx)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get the version for %v", req.Browser)
		}
		versions = append(versions, version)

	// TODO: Implement case proto.BrowserType_LACROS_STATEFUL when needed.
	default:
		return nil, errors.Errorf("unknown browser type: %v", req.Browser)
	}
	testing.ContextLogf(ctx, "GetBrowserVersion: type: %v, version: %v", req.Browser, versions)
	return &proto.GetBrowserVersionResponse{
		Versions: versions,
	}, nil
}

// getAshVersion returns non-empty version of Ash Chrome.
func (uts *UpdateTestService) getAshVersion(ctx context.Context) (string, error) {
	out, err := testexec.CommandContext(ctx, "/opt/google/chrome/chrome", "--version").Output(testexec.DumpLogOnError)
	if err != nil {
		return "", err
	}
	version := versionRegexp.FindString(string(out))
	if version == "" {
		return "", errors.New("invalid version")
	}
	return version, nil
}

// getLacrosRootfsVersion returns non-empty version of Lacros Chrome in Rootfs.
func (uts *UpdateTestService) getLacrosRootfsVersion(ctx context.Context) (string, error) {
	out, err := testexec.CommandContext(ctx, "cat", "/opt/google/lacros/metadata.json").Output(testexec.DumpLogOnError)
	if err != nil {
		return "", err
	}
	metadataJSON := lacrosMetadata{}
	if err := json.Unmarshal([]byte(out), &metadataJSON); err != nil {
		return "", errors.Wrap(err, "failed to parse Rootfs Lacros Chrome version")
	}
	version := versionRegexp.FindString(metadataJSON.Content.Version)
	if version == "" {
		return "", errors.New("invalid version")
	}
	return version, nil
}

// setupAshChrome configures Ash Chrome to be able to launch Lacros with given options.
// Note that it uses fake login credentials and loads test extension for Lacros by default.
func (uts *UpdateTestService) setupAshChrome(ctx context.Context, opts []chrome.Option) (*chrome.Chrome, error) {
	// We reuse the custom extension from the chrome package for exposing private interfaces.
	extDirs, err := chrome.DeprecatedPrepareExtensions()
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare extensions")
	}
	extList := strings.Join(extDirs, ",")
	opts = append(opts, chrome.LacrosExtraArgs(launcher.ExtensionArgs(chrome.TestExtensionID, extList)...))
	// KeepEnrollment should be enabled to retain user data including provisioned Lacros image
	// after restarting ui to make new Chrome options take effect.
	opts = append(opts, chrome.KeepEnrollment())

	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to Chrome")
	}
	return cr, nil
}

// getVersionInfoFromUI returns the version detail for a given expression on the chrome://version page.
// eg, Executable Path = /run/imageloader/lacros-dogfood-dev/X.X.X.X/chrome
func (uts *UpdateTestService) getVersionInfoFromUI(ctx context.Context, l *launcher.LacrosChrome, expr string) (string, error) {
	lconn, err := l.NewConn(ctx, versionURL)
	if err != nil {
		return "", errors.Wrapf(err, "failed to open: %v", versionURL)
	}
	defer lconn.Close()

	var value string
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var raw json.RawMessage
		if err := lconn.Eval(ctx, expr, &raw); err != nil {
			return errors.Wrapf(err, "failed to eval expr: %v", expr)
		}
		value = strings.Trim(string(raw), ` "`)
		if len(value) > 0 {
			return nil
		}
		return errors.New("failed to find value for eval")
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		return "", err
	}
	return value, nil
}
