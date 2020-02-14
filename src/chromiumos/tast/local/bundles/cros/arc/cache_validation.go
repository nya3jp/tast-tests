// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     CacheValidation,
		Desc:     "Validates that caches match for both modes when pre-generated packages cache is enabled and disabled",
		Contacts: []string{"arc-performance@google.com", "wvk@google.com", "khmel@google.com"},
		Attr:     []string{"group:mainline", "informational"},
		// TODO(wvk): enable this test for ARC++ after we build arccachevalidator for ARC++ images
		SoftwareDeps: []string{"android_vm", "chrome", "chrome_internal"},
		Timeout:      8 * time.Minute,
	})
}

const (
	packagesCacheXML = "packages_cache.xml"
	layoutTxt        = "layout.txt"
)

// packagesCacheMode represents a flag that determines whether packages_cache.xml
// will be copied within ARC after boot.
type packagesCacheMode int

const (
	copy packagesCacheMode = iota
	skipCopy
)

// pathCondition represents whether waitForAndroidPath should wait for the path
// to be created or to be removed.
type pathCondition int

const (
	pathMustExist pathCondition = iota
	pathMustNotExist
)

func CacheValidation(ctx context.Context, s *testing.State) {
	withCacheDir := filepath.Join(s.OutDir(), "with_cache")
	withoutCacheDir := filepath.Join(s.OutDir(), "without_cache")
	if err := os.Mkdir(withCacheDir, 0755); err != nil {
		s.Fatalf("Could not make output subdirectory %s: %v", withCacheDir, err)
	}
	if err := os.Mkdir(withoutCacheDir, 0755); err != nil {
		s.Fatalf("Could not make output subdirectory %s: %v", withoutCacheDir, err)
	}

	// Boot ARC with and without caches enabled, and copy relevant files to output directory.
	s.Log("Starting ARC, with packages cache disabled")
	if cr, a, err := bootARCWithPackagesCacheMode(ctx, s, skipCopy, withoutCacheDir); err != nil {
		s.Fatal("Booting ARC failed: ", err)
	} else {
		a.Close()
		cr.Close(ctx)
	}
	s.Log("Starting ARC, with packages cache enabled")
	cr, a, err := bootARCWithPackagesCacheMode(ctx, s, copy, withCacheDir)
	if err != nil {
		s.Fatal("Booting ARC failed: ", err)
	}
	defer cr.Close(ctx)
	defer a.Close()

	s.Log("Validating GMS Core cache")
	if err := saveOutput(filepath.Join(s.OutDir(), "app_chimera.diff"),
		testexec.CommandContext(ctx, "diff", "--recursive", "--no-dereference",
			filepath.Join(withCacheDir, "app_chimera"),
			filepath.Join(withoutCacheDir, "app_chimera"))); err != nil {
		s.Error("Error validating app_chimera folders: ", err)
	}
	if err := saveOutput(filepath.Join(s.OutDir(), "layout.diff"),
		testexec.CommandContext(ctx, "diff", filepath.Join(withCacheDir, layoutTxt),
			filepath.Join(withoutCacheDir, layoutTxt))); err != nil {
		s.Error("Error validating app_chimera layouts: ", err)
	}

	tmpdir, err := a.TempDir(ctx)
	if err != nil {
		s.Fatal("Could not create ARC temporary directory: ", err)
	}
	defer a.RemoveAll(ctx, tmpdir)

	packagesWithCache := filepath.Join(tmpdir, "packages_with_cache.xml")
	if err := a.PushFile(ctx, filepath.Join(withCacheDir, packagesCacheXML), packagesWithCache); err != nil {
		s.Fatalf("Could not push %s to Android: %v", packagesWithCache, err)
	}
	packagesWithoutCache := filepath.Join(tmpdir, "packages_without_cache.xml")
	if err := a.PushFile(ctx, filepath.Join(withoutCacheDir, packagesCacheXML), packagesWithoutCache); err != nil {
		s.Fatalf("Could not push %s to Android: %v", packagesWithoutCache, err)
	}
	generatedPackagesCache := filepath.Join("/system/etc", packagesCacheXML)
	if err := a.PullFile(ctx, generatedPackagesCache, filepath.Join(s.OutDir(), packagesCacheXML)); err != nil {
		s.Fatalf("Could not pull %s from Android, this may mean that pre-generated packages cache was not installed when building the image: %v", generatedPackagesCache, err)
	}

	s.Log("Validating packages cache")
	if err := saveOutput(filepath.Join(s.OutDir(), "arccachevalidator_dynamic.out"),
		a.Command(ctx, "arccachevalidator", "--source", packagesWithCache,
			"--reference", packagesWithoutCache, "--dynamic-validate", "yes")); err != nil {
		s.Error("packages_cache dynamic validation failed: ", err)
	}
	if err := saveOutput(filepath.Join(s.OutDir(), "arccachevalidator_nondynamic.out"),
		a.Command(ctx, "arccachevalidator", "--source", generatedPackagesCache,
			"--reference", packagesWithoutCache, "--dynamic-validate", "no")); err != nil {
		s.Error("packages_cache non-dynamic validation failed: ", err)
	}
}

// bootARCWithPackagesCacheMode boots ARC with caches turned on or off, depending on the mode parameter. In
// each case, resulting files (app_chimera, layout.txt) are copied to the specified output
// directory. On success, non-nil pointers are returned that must be closed by the calling function. However,
// if there is an error, both pointers will be nil.
func bootARCWithPackagesCacheMode(ctx context.Context, s *testing.State, mode packagesCacheMode, outputDir string) (cr *chrome.Chrome, a *arc.ARC, retErr error) {
	args := []string{"--arc-disable-app-sync", "--arc-disable=play-auto-install"}
	if mode == skipCopy {
		args = append(args, "--arc-packages-cache-mode=skip-copy")
	} else {
		args = append(args, "--arc-packages-cache-mode=copy")
	}
	cr, err := chrome.New(ctx, chrome.ARCEnabled(), chrome.ExtraArgs(args...))
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to login to Chrome")
	}
	defer func() {
		if retErr != nil && cr != nil {
			cr.Close(ctx)
		}
	}()

	a, err = arc.New(ctx, s.OutDir())
	if err != nil {
		return cr, nil, errors.Wrap(err, "could not start ARC")
	}
	defer func() {
		if retErr != nil && a != nil {
			a.Close()
		}
	}()

	const chimeraPath = "/data/user_de/0/com.google.android.gms/app_chimera"

	for _, e := range []struct {
		filename string
		cond     pathCondition
	}{
		{"current_config.fb", pathMustExist},
		{"current_fileapks.pb", pathMustExist},
		{"stored_modulesets.pb", pathMustExist},
		{"current_modules_init.pb", pathMustNotExist},
	} {
		if err := waitForAndroidPath(ctx, s, a, filepath.Join(chimeraPath, e.filename), e.cond); err != nil {
			return cr, a, err
		}
	}

	err = a.PullFile(ctx, chimeraPath, filepath.Join(outputDir, "app_chimera"))
	if err != nil {
		return cr, a, errors.Wrapf(err, "failed to pull %s from Android", chimeraPath)
	}

	out, err := a.Command(ctx, "find", "-L", chimeraPath, "-exec", "stat", "-Lc", "%n:%a:%b", "{}", "+").Output(testexec.DumpLogOnError)
	if err != nil {
		return cr, a, errors.Wrap(err, "failed to gather app_chimera file attributes")
	}
	lines := strings.Split(string(out), "\n")
	sort.Strings(lines)
	layout := strings.Join(lines, "\n")
	err = ioutil.WriteFile(filepath.Join(outputDir, layoutTxt), []byte(layout), 0644)
	if err != nil {
		return cr, a, errors.Wrapf(err, "failed to generate %s for %s", layoutTxt, chimeraPath)
	}

	const packagesPath = "/data/system/packages_copy.xml"
	err = a.PullFile(ctx, packagesPath, filepath.Join(outputDir, packagesCacheXML))
	if err != nil {
		return cr, a, errors.Wrapf(err, "failed to pull %s from Android", packagesPath)
	}

	return cr, a, nil
}

// waitForAndroidPath waits up to 1 minute or ctx deadline for the specified path to
// exist or not exist depending on pathCondition c.
func waitForAndroidPath(ctx context.Context, s *testing.State, a *arc.ARC, path string, c pathCondition) error {
	s.Logf("Waiting for Android path %s", path)
	err := testing.Poll(ctx, func(ctx context.Context) error {
		var comm *testexec.Cmd
		var msg string
		if c == pathMustExist {
			comm = a.Command(ctx, "test", "-e", path)
			msg = "does not exist"
		} else {
			comm = a.Command(ctx, "test", "!", "-e", path)
			msg = "exists"
		}
		if err := comm.Run(); err != nil {
			return errors.Wrapf(err, "path %s still %s", path, msg)
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  time.Minute,
		Interval: time.Second,
	})
	if err != nil {
		return errors.Wrapf(err, "failed to wait for path %s", path)
	}
	return nil
}

// saveOutput runs the command specified by name with args as arguments, and saves
// the stdout and stderr to outPath.
func saveOutput(outPath string, cmd *testexec.Cmd) error {
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()
	cmd.Stdout = f
	cmd.Stderr = f
	return cmd.Run(testexec.DumpLogOnError)
}
