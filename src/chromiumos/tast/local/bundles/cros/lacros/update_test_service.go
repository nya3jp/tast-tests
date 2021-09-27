// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacros

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"google.golang.org/grpc"

	lacroscommon "chromiumos/tast/common/cros/lacros"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/lacros/launcher"
	lacrosservice "chromiumos/tast/services/cros/lacros"
	"chromiumos/tast/testing"
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
			lacrosservice.RegisterUpdateTestServiceServer(srv, &UpdateTestService{s: s})
		},
	})
}

// UpdateTestService implements tast.cros.lacros.UpdateTestService.
type UpdateTestService struct {
	s *testing.ServiceState
}

// VerifyUpdate checks if the expected version of Lacros is loaded successfully without crash given the browsers provisioned.
func (uts *UpdateTestService) VerifyUpdate(ctx context.Context, req *lacrosservice.VerifyUpdateRequest) (*lacrosservice.VerifyUpdateResponse, error) {
	// Setup Ash Chrome with the given context including options and login info.
	cr, tconn, err := uts.setupChrome(ctx, req.AshContext.GetOpts())
	if err != nil {
		return nil, errors.Wrap(err, "failed to setup Ash Chrome")
	}
	defer func(ctx context.Context) error {
		if err := cr.Close(ctx); err != nil {
			return errors.Wrap(err, "failed to close Ash Chrome")
		}
		return nil
	}(ctx)

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
	status := lacrosservice.TestResult_NO_STATUS
	statusDetails := ""
	if req.UseUi {
		// Verify on the chrome://version UI that it has the expected version in "Executable Path".
		// Note that "Google Chrome" on the page cannot be used for verification since it is read from the binary
		// and does not reflect the version that is mocked for test in runtime.
		const exprExecutablePath = `document.querySelector('#executable_path').innerText`
		actualVersionedLacrosPath, err := uts.versionInfoFromUI(ctx, l, exprExecutablePath)
		if err != nil {
			return nil, errors.Wrap(err, "failed to read Lacros executable path")
		}
		if actualVersionedLacrosPath == expectedVersionedLacrosPath {
			status = lacrosservice.TestResult_PASSED
			statusDetails = fmt.Sprintf("executable path: %v", actualVersionedLacrosPath)
		} else {
			status = lacrosservice.TestResult_FAILED
			statusDetails = fmt.Sprintf("executable path expected: %v, actual: %v", expectedVersionedLacrosPath, actualVersionedLacrosPath)
		}
	} else {
		// Verify without UI that the lacros process is running from the expected versioned path.
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			pids, err := launcher.PidsFromPath(ctx, expectedVersionedLacrosPath)
			if err != nil {
				return err
			}
			if len(pids) > 0 {
				return nil
			}
			return errors.New("waiting for lacros process")
		}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
			return nil, errors.Wrap(err, "could not find running process")
		}
		status = lacrosservice.TestResult_PASSED
	}

	return &lacrosservice.VerifyUpdateResponse{
		Result: &lacrosservice.TestResult{Status: status, StatusDetails: statusDetails},
	}, nil
}

// ClearUpdate removes all provisioned Lacros on a DUT to reset to the previous state between tests.
func (uts *UpdateTestService) ClearUpdate(ctx context.Context, req *lacrosservice.ClearUpdateRequest) (*lacrosservice.ClearUpdateResponse, error) {
	testing.ContextLog(ctx, "Clearing provisioned Lacros")
	// Mark the stateful partition corrupt, so the provision can restore it.
	// Remove it only if the clean up is successful.
	if err := ioutil.WriteFile(lacroscommon.CorruptStatefulFilePath, nil, 0644); err != nil {
		testing.ContextLog(ctx, "Failed to touch file: ", err)
	}

	// Unmount and remove mount points.
	matches, _ := filepath.Glob("/run/imageloader/lacros*/*")
	for _, match := range matches {
		if err := testexec.CommandContext(ctx, "umount", "-f", match).Run(); err != nil {
			testing.ContextLog(ctx, "Failed to unmount ", match)
		}
		if err := os.RemoveAll(match); err != nil {
			testing.ContextLog(ctx, "Failed to remove ", match)
		}
	}

	// Remove provisioned files. Note that 'sh' is used to handle the glob.
	lacrosComponentPathGlob := filepath.Join(lacroscommon.LacrosRootComponentPath, "*")
	if err := testexec.CommandContext(ctx, "sh", "-c",
		strings.Join([]string{"rm", "-rf", lacrosComponentPathGlob}, " ")).Run(); err != nil {
		testing.ContextLog(ctx, "Failed to remove provisioned components at ", lacrosComponentPathGlob)
	}

	// If succeeded to clear, we no longer need to mark the stateful partition corrupt.
	matches, _ = filepath.Glob(lacrosComponentPathGlob)
	if len(matches) == 0 {
		os.Remove(lacroscommon.CorruptStatefulFilePath)
	}
	return &lacrosservice.ClearUpdateResponse{}, nil
}

// GetBrowserVersion returns version info of the given browser type.
// If multiple Lacros browsers are provisioned in the stateful partition, all the versions will be returned.
func (uts *UpdateTestService) GetBrowserVersion(ctx context.Context, req *lacrosservice.GetBrowserVersionRequest) (*lacrosservice.GetBrowserVersionResponse, error) {
	var versions []string
	switch req.Browser {
	case lacrosservice.BrowserType_ASH:
		version, err := uts.ashVersion(ctx)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get the version for %v", req.Browser)
		}
		versions = append(versions, version)

	case lacrosservice.BrowserType_LACROS_ROOTFS:
		version, err := uts.lacrosRootfsVersion(ctx)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get the version for %v", req.Browser)
		}
		versions = append(versions, version)

	// TODO: Implement case lacrosservice.BrowserType_LACROS_STATEFUL when needed.
	default:
		return nil, errors.Errorf("unknown browser type: %v", req.Browser)
	}
	testing.ContextLogf(ctx, "GetBrowserVersion: type: %v, version: %v", req.Browser, versions)
	return &lacrosservice.GetBrowserVersionResponse{
		Versions: versions,
	}, nil
}

// ashVersion returns non-empty version of Ash Chrome.
// TODO(hyungtaekim): Move the function to a common place for other tests.
func (uts *UpdateTestService) ashVersion(ctx context.Context) (string, error) {
	out, err := testexec.CommandContext(ctx, "/opt/google/chrome/chrome", "--version").Output(testexec.DumpLogOnError)
	if err != nil {
		return "", err
	}
	version := versionRegexp.FindString(string(out))
	if version == "" {
		return "", errors.New("invalid version: " + version)
	}
	return version, nil
}

// lacrosRootfsVersion returns non-empty version of Lacros Chrome in Rootfs.
// TODO(hyungtaekim): Move the function to a common place for other tests.
func (uts *UpdateTestService) lacrosRootfsVersion(ctx context.Context) (string, error) {
	metadata, err := ioutil.ReadFile("/opt/google/lacros/metadata.json")
	if err != nil {
		return "", err
	}
	metadataJSON := lacrosMetadata{}
	if err := json.Unmarshal(metadata, &metadataJSON); err != nil {
		return "", errors.Wrap(err, "failed to parse Rootfs Lacros Chrome version")
	}
	version := versionRegexp.FindString(metadataJSON.Content.Version)
	if version == "" {
		return "", errors.New("invalid version: " + version)
	}
	return version, nil
}

// setupChrome configures Ash Chrome to be able to launch Lacros with given options.
// Note that it uses fake login credentials and loads test extension for Lacros by default.
func (uts *UpdateTestService) setupChrome(ctx context.Context, options []string) (*chrome.Chrome, *chrome.TestConn, error) {
	var opts []chrome.Option
	for _, opt := range options {
		opts = append(opts, chrome.Option(chrome.ExtraArgs(opt)))
	}

	// We reuse the custom extension from the chrome package for exposing private interfaces.
	extDirs, err := chrome.DeprecatedPrepareExtensions()
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to prepare extensions")
	}
	extList := strings.Join(extDirs, ",")
	opts = append(opts, chrome.LacrosExtraArgs(launcher.ExtensionArgs(chrome.TestExtensionID, extList)...))
	// KeepEnrollment should be enabled to retain user data including provisioned Lacros image
	// after restarting ui to make new Chrome options take effect.
	opts = append(opts, chrome.KeepEnrollment())

	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to connect to Chrome")
	}
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to create test API connection for Ash Chrome")
	}
	return cr, tconn, nil
}

// versionInfoFromUI opens chrome://version/ from UI and returns the version info that matches the given JS expression.
// eg, Executable Path = /run/imageloader/lacros-dogfood-dev/X.X.X.X/chrome
func (uts *UpdateTestService) versionInfoFromUI(ctx context.Context, l *launcher.LacrosChrome, expr string) (string, error) {
	lconn, err := l.NewConn(ctx, lacroscommon.VersionURL)
	if err != nil {
		return "", errors.Wrapf(err, "failed to open: %v", lacroscommon.VersionURL)
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
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		return "", err
	}
	return value, nil
}
