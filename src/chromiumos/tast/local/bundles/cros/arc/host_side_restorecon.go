// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	goselinux "github.com/opencontainers/selinux/go-selinux"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         HostSideRestorecon,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that host-side restorecon does not take effect on ARC",
		Contacts:     []string{"youkichihosoi@chromium.org", "arc-storage@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		Timeout: 12 * time.Minute,
		VarDeps: []string{"ui.gaiaPoolDefault"},
	})
}

func HostSideRestorecon(ctx context.Context, s *testing.State) {
	creds, err := chrome.PickRandomCreds(s.RequiredVar("ui.gaiaPoolDefault"))
	if err != nil {
		s.Fatal("Failed to pick random credentials: ", err)
	}

	// Components of paths to check SELinux labels.
	pathsComponents := [][]string{
		{"data", "media", "0", "Pictures"},
		{"data", "misc", "adb", "adb_keys"},
		{"data", "data", "com.android.providers.downloads", "databases", "downloads.db"},
	}

	correctLabels := setUpHostSideRestorecon(ctx, s, creds, pathsComponents)
	testHostSideRestorecon(ctx, s, creds, pathsComponents, correctLabels)
}

// setUpHostSideRestorecon invalidates security.sehash xattr of directories
// between /home/.shadow/<hash> and the paths specified by pathsComponents so
// that host-side restorecon can take effect in the next user session.
// Its return value is a map from file paths to their SELinux labels. The map's
// domain is the files between /home/root/<hash>/android-data/data and the paths
// specified by pathsComponents. It can be used to check whether the SELinux
// labels of these files are correct in the next user session.
func setUpHostSideRestorecon(ctx context.Context, s *testing.State, creds chrome.Creds, pathsComponents [][]string) map[string]string {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cr, err := chrome.New(ctx,
		chrome.GAIALogin(creds),
		chrome.ARCSupported(),
		chrome.ExtraArgs(arc.DisableSyncFlags()...))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	performOptin(ctx, s, cr)

	vaultPath, err := cryptohome.MountedVaultPath(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatalf("Failed to get mounted vault path for %v: %v", cr.NormalizedUser(), err)
	}

	// Invalidate security.sehash xattr of directories between /home/.shadow/<hash>
	// and /home/.shadow/<hash>/mount/root/android-data.
	invalidateSELinuxHash(ctx, s, filepath.Dir(vaultPath))
	invalidateSELinuxHash(ctx, s, vaultPath)
	invalidateSELinuxHash(ctx, s, filepath.Join(vaultPath, "root"))
	invalidateSELinuxHash(ctx, s, filepath.Join(vaultPath, "root", "android-data"))

	return getSELinuxLabelsUnderAndroidData(ctx, s, cr, pathsComponents)
}

func performOptin(ctx context.Context, s *testing.State, cr *chrome.Chrome) {
	s.Log("Performing optin")
	const maxAttempts = 2
	if err := optin.PerformWithRetry(ctx, cr, maxAttempts); err != nil {
		s.Fatal("Failed to optin: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	if err := optin.WaitForPlayStoreShown(ctx, tconn, time.Minute); err != nil {
		s.Fatal("Failed to wait for Play Store: ", err)
	}
}

func invalidateSELinuxHash(ctx context.Context, s *testing.State, path string) {
	// Instead of removing security.sehash xattr, set its value to be an empty
	// string. This is because "setfattr -x security.sehash" fails when the
	// attribute has no value.
	cmd := testexec.CommandContext(ctx, "setfattr", "-n", "security.sehash", "-v", "\"\"", path)
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		s.Fatalf("Failed to invalidate security.sehash in %v: %v", path, err)
	}
}

func getSELinuxLabelsUnderAndroidData(ctx context.Context, s *testing.State, cr *chrome.Chrome, pathsComponents [][]string) map[string]string {
	androidDataDir, err := arc.AndroidDataDir(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatalf("Failed to get android data directory for %v: %v", cr.NormalizedUser(), err)
	}

	// Get the SELinux labels of files between .../android-data/data and the paths
	// specified by pathsComponents. Also invalidate their security.sehash xattr.
	labels := make(map[string]string)
	for _, components := range pathsComponents {
		path := androidDataDir
		for _, entry := range components {
			path = filepath.Join(path, entry)
			if labels[path] != "" {
				continue
			}

			invalidateSELinuxHash(ctx, s, path)

			label, err := getSELinuxLabel(path)
			if err != nil {
				s.Fatalf("Failed to get SELinux label of %v: %v", path, err)
			}
			labels[path] = label
		}
	}
	return labels
}

func getSELinuxLabel(path string) (string, error) {
	label, err := goselinux.FileLabel(path)
	if err != nil {
		return "", err
	}

	// A complete SELinux context (label) is a colon-separated string of the form
	// <user>:<role>:<type>:<level>, e.g.:
	//   u:object_r:media_rw_data_file:s0
	//   u:object_r:privapp_data_file:s0:c512,c768
	// Here, we extract the <type> part only. This is because a full label
	// comparison does not work well for ARC++ P, where <level> can change after
	// label restoration.
	components := strings.Split(label, ":")
	if len(components) < 4 || components[0] != "u" || components[1] != "object_r" {
		return "", errors.Errorf("unexpected format of SELinux label: %v", label)
	}
	return components[2], nil
}

// testHostSideRestorecon checks that the SELinux labels of the files specified
// by pathsComponents are still correct in a new user session. After that, it
// also checks whether an Android app can be installed.
func testHostSideRestorecon(ctx context.Context, s *testing.State, creds chrome.Creds, pathsComponents [][]string, correctLabels map[string]string) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Restart a user session with the KeepState option.
	cr, err := chrome.New(ctx,
		chrome.GAIALogin(creds),
		chrome.ARCSupported(),
		chrome.KeepState(),
		chrome.ExtraArgs(arc.DisableSyncFlags()...))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close(cleanupCtx)

	checkSELinuxLabelsUnderAndroidData(ctx, s, cr, pathsComponents, correctLabels)

	checkPlayability(ctx, s, a)
}

func checkSELinuxLabelsUnderAndroidData(ctx context.Context, s *testing.State, cr *chrome.Chrome, pathsComponents [][]string, correctLabels map[string]string) {
	androidDataDir, err := arc.AndroidDataDir(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatalf("Failed to get android data directory for %v: %v", cr.NormalizedUser(), err)
	}

	// Check that the files between /home/root/<hash>/android-data/data and the
	// paths specified by pathsComponents have the correct SELinux labels.
	for _, components := range pathsComponents {
		path := androidDataDir
		for _, entry := range components {
			path = filepath.Join(path, entry)
			actual, err := getSELinuxLabel(path)
			if err != nil {
				s.Fatalf("Failed to get SELinux label of %v: %v", path, err)
			}

			expected := correctLabels[path]
			if actual != expected {
				s.Fatalf("Incorrect SELinux label for %v: actual: %v, expected: %v", path, actual, expected)
			}
		}
	}
}

func checkPlayability(ctx context.Context, s *testing.State, a *arc.ARC) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed to initialize UI Automator: ", err)
	}
	defer d.Close(cleanupCtx)

	s.Log("Installing Android app")
	const pkgName = "com.google.android.calculator"
	if err := playstore.InstallApp(ctx, a, d, pkgName, &playstore.Options{TryLimit: -1}); err != nil {
		s.Fatal("Failed to install Android app: ", err)
	}
}
