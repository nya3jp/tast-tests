// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package apps

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/testing"
)

const (
	indexHTML = "telemetry_extension.html"
	indexJS   = "telemetry_extension.js"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: TelemetryExtension,
		Desc: "Launches TelemetryExtension and requests telemetry data",
		Contacts: []string{
			"lamzin@google.com", // Test and TelemetryExtension author
			"mgawad@google.com", // TelemetryExtension author
		},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{indexHTML, indexJS},
	})
}

// TelemetryExtension tests that TelemetryExtension can be launched and
// telemetry API is working.
func TelemetryExtension(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	dir, err := ioutil.TempDir("", "telemetry_extension")
	if err != nil {
		s.Fatal("Failed to create temporary directory for TelemetryExtension: ", err)
	}
	defer os.RemoveAll(dir)

	if err := fsutil.CopyFile(s.DataPath(indexHTML), filepath.Join(dir, "index.html")); err != nil {
		s.Fatalf("Failed to copy %q file to %q: %v", indexHTML, dir, err)
	}
	if err := fsutil.CopyFile(s.DataPath(indexJS), filepath.Join(dir, "index.js")); err != nil {
		s.Fatalf("Failed to copy %q file to %q: %v", indexJS, dir, err)
	}

	if err := os.Chown(dir, int(sysutil.ChronosUID), int(sysutil.ChronosGID)); err != nil {
		s.Fatal("Failed to chown root dir: ", err)
	}
	if err := os.Chown(filepath.Join(dir, "index.html"), int(sysutil.ChronosUID), int(sysutil.ChronosGID)); err != nil {
		s.Fatal("Failed to chown index.html: ", err)
	}
	if err := os.Chown(filepath.Join(dir, "index.js"), int(sysutil.ChronosUID), int(sysutil.ChronosGID)); err != nil {
		s.Fatal("Failed to chown index.js: ", err)
	}

	cr, err := chrome.New(ctx,
		chrome.EnableFeatures("TelemetryExtension"),
		chrome.ExtraArgs(fmt.Sprintf("--telemetry-extension-dir=%s", dir)))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	if err := ash.WaitForChromeAppInstalled(ctx, tconn, apps.TelemetryExtension.ID, 2*time.Minute); err != nil {
		s.Fatal("Failed to wait TelemetryExtension to install: ", err)
	}

	s.Log("Launching TelemetryExtension")
	if err := apps.Launch(ctx, tconn, apps.TelemetryExtension.ID); err != nil {
		s.Fatal("Failed to launch Telemetry Extension: ", err)
	}
	defer apps.Close(cleanupCtx, tconn, apps.TelemetryExtension.ID)

	if err := ash.WaitForApp(ctx, tconn, apps.TelemetryExtension.ID, time.Minute); err != nil {
		s.Fatalf("Failed to wait for %q by app id %q: %v", apps.TelemetryExtension.Name, apps.TelemetryExtension.ID, err)
	}

	params := ui.FindParams{
		ClassName: "telemetryExtension",
		Role:      ui.RoleTypeHeading,
	}
	node, err := ui.FindWithTimeout(ctx, tconn, params, 10*time.Second)
	if err != nil {
		s.Fatal("Failed to find TelemetryExtension heading: ", err)
	}
	defer node.Release(cleanupCtx)

	var apiStatus, apiResponse string
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		status, err := preText(ctx, tconn, "telemetryApiStatus")
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get telemetry API status"))
		}

		if status == "" {
			return errors.New("telemetry API status is empty")
		}

		response, err := preText(ctx, tconn, "telemetryApiResponse")
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get telemetry API response"))
		}

		apiStatus = status
		apiResponse = response
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		s.Fatal("Failed to wait telemetry status: ", err)
	}

	const responseFile = "telemetry_api_response.txt"
	s.Logf("Saving telemetry API response into %q", responseFile)
	if err := ioutil.WriteFile(filepath.Join(s.OutDir(), responseFile), []byte(apiResponse), 0666); err != nil {
		s.Errorf("Failed to save telemetry API response into %q: %v", responseFile, err)
	}

	if apiStatus != "Success" {
		s.Errorf("Telemetry API failed, check reason in %q", responseFile)
	}
}

func preText(ctx context.Context, tconn *chrome.TestConn, className string) (string, error) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Second)
	defer cancel()

	params := ui.FindParams{
		ClassName: className,
		Role:      ui.RoleTypePre,
	}
	node, err := ui.FindWithTimeout(ctx, tconn, params, time.Second)
	if err != nil {
		return "", errors.Wrap(err, "failed to find pre node")
	}
	defer node.Release(cleanupCtx)

	children, err := node.Children(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to get children for pre node")
	}
	defer children.Release(cleanupCtx)

	if len(children) != 1 {
		return "", errors.Errorf("unexpected number of children: want 1; got %d", len(children))
	}

	// `textContent` attribute of `pre` HTML tag will appear as `name` attribute
	// of the first (and only one) child of `pre` node in the accessibility tree.
	return children[0].Name, nil
}
