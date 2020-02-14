// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/arc/cache"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     CacheValidation,
		Desc:     "Validates that caches match for both modes when pre-generated packages cache is enabled and disabled",
		Contacts: []string{"arc-performance@google.com", "wvk@google.com", "khmel@google.com"},
		Attr:     []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p", "chrome"},
			Val:               []string{},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm", "chrome"},
			Val:               []string{"--enable-arcvm"},
		}},
		Timeout: 8 * time.Minute,
	})
}

func CacheValidation(ctx context.Context, s *testing.State) {
	extraArgs := s.Param().([]string)
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
	if cr, a, err := cache.OpenSession(ctx, cache.SkipCopy, extraArgs, withoutCacheDir); err != nil {
		s.Fatal("Booting ARC failed: ", err)
	} else {
		err := cache.CopyCaches(ctx, a, withoutCacheDir)
		cr.Close(ctx)
		a.Close()
		if err != nil {
			s.Fatal("Copying caches failed: ", err)
		}
	}

	s.Log("Starting ARC, with packages cache enabled")
	cr, a, err := cache.OpenSession(ctx, cache.Copy, extraArgs, withCacheDir)
	if err != nil {
		s.Fatal("Booting ARC failed: ", err)
	}
	defer cr.Close(ctx)
	defer a.Close()

	if err := cache.CopyCaches(ctx, a, withCacheDir); err != nil {
		s.Fatal("Copying caches failed: ", err)
	}

	// unpackGmsCoreCaches unpack GMS core caches which are returned by cache.BootARC packed
	// in tar.
	unpackGmsCoreCaches := func(outputDirs []string) error {
		for _, outputDir := range outputDirs {
			tarPath := filepath.Join(outputDir, cache.GmsCoreCacheArchive)
			if err = testexec.CommandContext(
				ctx, "tar", "-xvpf", tarPath, "-C", outputDir).Run(); err != nil {
				return errors.Wrapf(err, "decompression %q failed", tarPath)
			}
			if err = os.Remove(tarPath); err != nil {
				return errors.Wrapf(err, "failed to cleanup %q", tarPath)
			}
		}

		return nil
	}
	if err = unpackGmsCoreCaches([]string{withCacheDir, withoutCacheDir}); err != nil {
		s.Fatal("Could not prepare GMS Core caches: ", err)
	}

	// saveOutput runs the command specified by name with args as arguments, and saves
	// the stdout and stderr to outPath.
	saveOutput := func(outPath string, cmd *testexec.Cmd) error {
		f, err := os.Create(outPath)
		if err != nil {
			return err
		}
		defer f.Close()
		cmd.Stdout = f
		cmd.Stderr = f
		return cmd.Run(testexec.DumpLogOnError)
	}

	s.Log("Validating GMS Core cache")
	if err := saveOutput(filepath.Join(s.OutDir(), "app_chimera.diff"),
		testexec.CommandContext(ctx, "diff", "--recursive", "--no-dereference",
			filepath.Join(withCacheDir, "app_chimera"),
			filepath.Join(withoutCacheDir, "app_chimera"))); err != nil {
		s.Error("Error validating app_chimera folders: ", err)
	}
	if err := saveOutput(filepath.Join(s.OutDir(), "layout.diff"),
		testexec.CommandContext(ctx, "diff", filepath.Join(withCacheDir, cache.LayoutTxt),
			filepath.Join(withoutCacheDir, cache.LayoutTxt))); err != nil {
		s.Error("Error validating app_chimera layouts: ", err)
	}

	tmpdir, err := a.TempDir(ctx)
	if err != nil {
		s.Fatal("Could not create ARC temporary directory: ", err)
	}
	defer a.RemoveAll(ctx, tmpdir)

	packagesWithCache := filepath.Join(tmpdir, "packages_with_cache.xml")
	if err := a.PushFile(ctx, filepath.Join(withCacheDir, cache.PackagesCacheXML), packagesWithCache); err != nil {
		s.Fatalf("Could not push %s to Android: %v", packagesWithCache, err)
	}
	packagesWithoutCache := filepath.Join(tmpdir, "packages_without_cache.xml")
	if err := a.PushFile(ctx, filepath.Join(withoutCacheDir, cache.PackagesCacheXML), packagesWithoutCache); err != nil {
		s.Fatalf("Could not push %s to Android: %v", packagesWithoutCache, err)
	}
	generatedPackagesCache := filepath.Join("/system/etc", cache.PackagesCacheXML)
	if err := a.PullFile(ctx, generatedPackagesCache, filepath.Join(s.OutDir(), cache.PackagesCacheXML)); err != nil {
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
