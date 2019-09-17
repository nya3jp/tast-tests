// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/graphics"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

// drmTest is used to describe the config used to run each drm_test.
type drmTest struct {
	command []string      // The command path to be run. This should be relative to /usr/local/bin.
	timeout time.Duration // Timeout to run the drmTest.
	runDeps []string      // The dependencies that should be check in runtime.
}

func init() {
	var (
		atomic       = drmTest{command: []string{"atomictest", "-a", "-t", "all"}, timeout: 5 * time.Minute, runDeps: []string{"display_connected", "drm_atomic"}}
		drmCursor    = drmTest{command: []string{"drm_cursor_test"}, timeout: 20 * time.Second, runDeps: []string{"display_connected"}}
		linearBo     = drmTest{command: []string{"linear_bo_test"}, timeout: 20 * time.Second, runDeps: []string{"display_connected"}}
		mmap         = drmTest{command: []string{"mmap_test"}, timeout: 5 * time.Minute, runDeps: []string{"display_connected"}}
		nullPlatform = drmTest{command: []string{"null_platform_test"}, timeout: 20 * time.Second, runDeps: []string{"display_connected"}}
		swrast       = drmTest{command: []string{"swrast_test"}, timeout: 20 * time.Second, runDeps: []string{}}
		vgem         = drmTest{command: []string{"vgem_test"}, timeout: 20 * time.Second, runDeps: []string{"display_connected"}}
		vkGlow       = drmTest{command: []string{"vk_glow"}, timeout: 20 * time.Second, runDeps: []string{"display_connected", "vulkan"}}
	)

	testing.AddTest(&testing.Test{
		Func: DRM,
		Desc: "Verifies DRM-related test binaries run successfully",
		Contacts: []string{
			"andrescj@chromium.org",
			"chromeos-gfx@google.com",
			"hidehiko@chromium.org", // Tast port.
		},
		Params: []testing.Param{{
			Name:              "atomic_test",
			Val:               []drmTest{atomic},
			ExtraSoftwareDeps: []string{"drm_atomic"},
		}, {
			Name: "drm_cursor_test",
			Val:  []drmTest{drmCursor},
		}, {
			Name: "linear_bo_test",
			Val:  []drmTest{linearBo},
		}, {
			Name: "mmap_test",
			Val:  []drmTest{mmap},
		}, {
			Name: "null_platform_test",
			Val:  []drmTest{nullPlatform},
		}, {
			Name: "swrast_test",
			Val:  []drmTest{swrast},
		}, {
			Name: "vgem_test",
			Val:  []drmTest{vgem},
		}, {
			Name:              "vk_glow",
			Val:               []drmTest{vkGlow},
			ExtraSoftwareDeps: []string{"vulkan"},
		}},
		Attr:    []string{"informational"},
		Timeout: 5 * time.Minute,
	})
}

func DRM(ctx context.Context, s *testing.State) {
	if err := setUp(ctx); err != nil {
		s.Fatal("Failed to set up the DRM test: ", err)
	}
	defer tearDown(ctx)

	testOpts := s.Param().([]drmTest)
	for _, testOpt := range testOpts {
		shouldRun, err := checkRunAttr(ctx, s.SoftwareDeps(), testOpt.runDeps)
		if err != nil {
			s.Error("Failed to check attribute: ", err)
		}
		if !shouldRun {
			continue
		}
		runTest(ctx, s, testOpt.timeout, testOpt.command[0], testOpt.command[1:]...)
	}
}

// checkRunAttr checks the runDeps config to decide whether we should run the test.
// TODO(pwang): runtime dependency is not recommended in tast. Hardware Dependency is currently blocked by crbug.com/950346. We should consider using it once it is done.
func checkRunAttr(ctx context.Context, softDeps, attrs []string) (bool, error) {
	for _, attr := range attrs {
		switch attr {
		case "display_connected":
			connectCount, err := graphics.NumberOfOutputsConnected(ctx)
			if err != nil {
				return false, errors.Wrapf(err, "failed to check test attribute %s", attr)
			}
			if connectCount == 0 {
				testing.ContextLog(ctx, "No connector detected. Skipping test")
				return false, nil
			}
		case "vulkan":
			hasVulkan, err := graphics.SupportsVulkanForDEQP(ctx)
			if err != nil {
				return false, errors.Wrapf(err, "failed to check test attribute %s", attr)
			}
			if !hasVulkan {
				for _, softDep := range softDeps {
					if softDep == "vulkan" {
						return false, errors.New("vulkan is not available but softDeps is set")
					}
				}
				testing.ContextLog(ctx, "Vulkan is not available. Skipping test")
				return false, nil
			}
		default:
			testing.ContextLogf(ctx, "Unrocognized runAttr %s. Assuming it is guarded by tast SoftwareDeps", attr)
		}
	}
	return true, nil
}

// setUp prepares the testing environment to run runTest().
func setUp(ctx context.Context) error {
	testing.ContextLog(ctx, "Setting up DRM test")
	return upstart.StopJob(ctx, "ui")
}

// tearDown restores the working environment after runTest().
func tearDown(ctx context.Context) error {
	testing.ContextLog(ctx, "Tearing down DRM test")
	return upstart.EnsureJobRunning(ctx, "ui")
}

// runTest runs the exe binary test. This method may be called several times as long as setUp() has been invoked beforehand.
func runTest(ctx context.Context, s *testing.State, t time.Duration, exe string, args ...string) {
	s.Log("Running ", shutil.EscapeSlice(append([]string{exe}, args...)))

	f, err := os.Create(filepath.Join(s.OutDir(), filepath.Base(exe)+".txt"))
	if err != nil {
		s.Fatal("Failed to create a log file: ", err)
	}
	defer f.Close()

	ctx, cancel := context.WithTimeout(ctx, t)
	defer cancel()
	cmd := testexec.CommandContext(ctx, exe, args...)
	cmd.Stdout = f
	cmd.Stderr = f
	if err := cmd.Run(); err != nil {
		s.Errorf("Failed to run %s: %v", exe, err)
	}
}
