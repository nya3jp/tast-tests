// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"path/filepath"
	"time"

	goselinux "github.com/opencontainers/selinux/go-selinux"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         HostSideRestorecon,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that host-side restorecon does not take effect on ARC",
		Contacts:     []string{"youkichihosoi@chromium.org", "arc-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		Timeout: 10 * time.Minute,
	})
}

func HostSideRestorecon(ctx context.Context, s *testing.State) {
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Check SELinux labels of files between .../android-data/data and
	// .../data/data/com.android.providers.downloads/databases/downloads.db.
	targetPathComponents := []string{
		"data",
		"data",
		"com.android.providers.downloads",
		"databases",
		"downloads.db",
	}

	correctLabels := setUpHostSideRestorecon(ctx, s, targetPathComponents)
	testHostSideRestorecon(ctx, s, targetPathComponents, correctLabels)
}

// setUpHostSideRestorecon invalidates security.sehash xattr of directories
// between /home/.shadow/<hash> and the parent of the path specified by
// targetPathComponents so that host-side restorecon can take effect in the next
// user session.
// Its return value is a map from file paths to their SELinux labels. The map's
// domain is the files between /home/root/<hash>/android-data/data and the path
// specified by targetPathComponents.
func setUpHostSideRestorecon(ctx context.Context, s *testing.State, targetPathComponents []string) map[string]string {
	cr, err := chrome.New(ctx, chrome.ARCEnabled())
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer func() {
		if err := cr.Close(ctx); err != nil {
			s.Fatal("Failed to close Chrome: ", err)
		}
	}()

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer func() {
		if a != nil {
			a.Close(ctx)
		}
	}()

	vaultPath, err := cryptohome.MountedVaultPath(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatalf("Failed to get mounted vault path for %v: %v", cr.NormalizedUser(), err)
	}

	// Invalidate security.sehash xattr of directories between /home/.shadow/<hash>
	// and /home/.shadow/<hash>/mount/root.
	invalidateSELinuxHash(ctx, s, filepath.Dir(vaultPath))
	invalidateSELinuxHash(ctx, s, vaultPath)
	invalidateSELinuxHash(ctx, s, filepath.Join(vaultPath, "root"))

	return getSELinuxLabelsUnderAndroidData(ctx, s, cr, targetPathComponents)
}

func invalidateSELinuxHash(ctx context.Context, s *testing.State, path string) {
	// Instead of removing security.sehash xattr, set its value to be an empty
	// string, because "setfattr -x security.sehash" fails when the attribute has
	// no value.
	cmd := testexec.CommandContext(ctx, "setfattr", "-n", "security.sehash", "-v", "\"\"", path)
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		s.Fatalf("Failed to clear security.sehash in %v: %v", path, err)
	}
}

func getSELinuxLabelsUnderAndroidData(ctx context.Context, s *testing.State, cr *chrome.Chrome, targetPathComponents []string) map[string]string {
	androidDataDir, err := arc.AndroidDataDir(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatalf("Failed to get android data directory for %v: %v", cr.NormalizedUser(), err)
	}

	// Invalidate security.sehash xattr of directories between .../android-data and
	// the parent of the path specified by targetComponents.
	// Also, get the SELinux labels of files between .../android-data/data and
	// the path specified by targetComponents.
	path := androidDataDir
	labels := make(map[string]string)
	for _, entry := range targetPathComponents {
		invalidateSELinuxHash(ctx, s, path)

		path = filepath.Join(path, entry)
		label, err := goselinux.FileLabel(path)
		if err != nil {
			s.Fatalf("Failed to get SELinux label of %v: %v", path, err)
		}
		labels[path] = label
	}

	return labels
}

// testHostSideRestorecon checks that SELinux labels of files under android-data
// are still correct in a new user session.
func testHostSideRestorecon(ctx context.Context, s *testing.State, targetPathComponents []string, correctLabels map[string]string) {
	// Restart a user session with the KeepState option.
	cr, err := chrome.New(ctx, chrome.ARCEnabled(), chrome.KeepState())
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer func() {
		if err := cr.Close(ctx); err != nil {
			s.Fatal("Failed to close Chrome: ", err)
		}
	}()

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer func() {
		if a != nil {
			a.Close(ctx)
		}
	}()

	checkSELinuxLabelsUnderAndroidData(ctx, s, cr, targetPathComponents, correctLabels)
}

func checkSELinuxLabelsUnderAndroidData(ctx context.Context, s *testing.State, cr *chrome.Chrome, targetPathComponents []string, correctLabels map[string]string) {
	androidDataDir, err := arc.AndroidDataDir(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatalf("Failed to get android data directory for %v: %v", cr.NormalizedUser(), err)
	}

	// Check that files between /home/root/<hash>/android-data/data and the
	// path specified by targetPathComponents have the correct SELinux labels.
	path := androidDataDir
	for _, entry := range targetPathComponents {
		path = filepath.Join(path, entry)
		actual, err := goselinux.FileLabel(path)
		if err != nil {
			s.Fatalf("Failed to get SELinux label of %v: %v", path, err)
		}

		expected := correctLabels[path]
		if actual != expected {
			s.Fatalf("Incorrect label for %v: actual: %v, expected: %v", path, actual, expected)
		}
	}
}
