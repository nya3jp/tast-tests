// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cache

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// PackagesMode represents a flag that determines whether packages_cache.xml
// will be copied within ARC after boot.
type PackagesMode int

const (
	// PackagesCopy forces to set caches and copy packages cache to the preserved destination.
	PackagesCopy PackagesMode = iota
	// PackagesSkipCopy forces to ignore caches and copy packages cache to the preserved destination.
	PackagesSkipCopy
)

// GMSCoreMode represents a flag that determines whether existing GMS Core caches
// would be used or not.
type GMSCoreMode int

const (
	// GMSCoreEnabled requires to use existing GMS Core caches if they are available.
	GMSCoreEnabled GMSCoreMode = iota
	// GMSCoreDisabled requires not to use existing GMS Core caches.
	GMSCoreDisabled
)

// pathCondition represents whether waitForAndroidPath should wait for the path
// to be created or to be removed.
type pathCondition int

const (
	pathMustExist pathCondition = iota
	pathMustNotExist
)

const (
	// LayoutTxt defines output file name that contains generated file directory layout and
	// file attributes.
	LayoutTxt = "layout.txt"
	// PackagesCacheXML defines the name of packages cache file name.
	PackagesCacheXML = "packages_cache.xml"
	// GeneratedPackagesCacheXML defines the name of pregenerated packages cache file name.
	// Used to rename the cache file retrieved from /system/etc.
	GeneratedPackagesCacheXML = "generated_packages_cache.xml"
	// GMSCoreCacheArchive defines the GMS Core cache tar file name.
	GMSCoreCacheArchive = "gms_core_cache.tar"
	// GMSCoreManifest defines the GMS Core manifest file that lists GMS Core release files
	// with size, modification time in milliseconds, and SHA256.
	// of GMS Core release.
	GMSCoreManifest = "gms_core_manifest"
	// GSFCache defines the GSF cache database file name.
	GSFCache = "gservices_cache.db"
)

// OpenSession starts Chrome and ARC with caches turned on or off, depending on the mode parameter.
// On success, non-nil pointers are returned that must be closed by the calling function.
// However, if there is an error, both pointers will be nil
func OpenSession(ctx context.Context, packagesMode PackagesMode, gmsCoreMode GMSCoreMode, extraArgs []string, outputDir string) (cr *chrome.Chrome, a *arc.ARC, retErr error) {
	args := []string{"--arc-disable-app-sync", "--arc-disable=play-auto-install"}
	switch packagesMode {
	case PackagesSkipCopy:
		args = append(args, "--arc-packages-cache-mode=skip-copy")
	case PackagesCopy:
		args = append(args, "--arc-packages-cache-mode=copy")
	default:
		return nil, nil, errors.Errorf("invalid packagesMode %d passed", packagesMode)
	}
	switch gmsCoreMode {
	case GMSCoreEnabled:
	case GMSCoreDisabled:
		args = append(args, "--arc-disable-gms-core-cache")
	default:
		return nil, nil, errors.Errorf("invalid gmsCoreMode %d passed", gmsCoreMode)
	}
	args = append(args, extraArgs...)

	// Signs in as chrome.DefaultUser.
	cr, err := chrome.New(ctx, chrome.ARCEnabled(), chrome.ExtraArgs(args...))
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to login to Chrome")
	}

	a, err = arc.New(ctx, outputDir)
	if err != nil {
		cr.Close(ctx)
		return nil, nil, errors.Wrap(err, "could not start ARC")
	}
	return cr, a, nil
}

// CopyCaches waits for required caches are ready and copies them to the specified output directory.
func CopyCaches(ctx context.Context, a *arc.ARC, outputDir string) error {
	const (
		gmsRoot      = "/data/user_de/0/com.google.android.gms"
		appChimera   = "app_chimera"
		packagesPath = "/data/system/packages_copy.xml"
		gsfDatabase  = "/data/data/com.google.android.gsf/databases/gservices.db"
	)

	// OpenSession signs in as chrome.DefaultUser.
	androidDataDir, err := arc.AndroidDataDir(chrome.DefaultUser)
	if err != nil {
		return errors.Wrap(err, "failed to get android-data path")
	}

	gmsRootUnderHome := filepath.Join(androidDataDir, gmsRoot)
	chimeraPath := filepath.Join(gmsRootUnderHome, appChimera)
	for _, e := range []struct {
		filename string
		cond     pathCondition
	}{
		{"current_config.fb", pathMustExist},
		{"current_fileapks.pb", pathMustExist},
		{"current_features.fb", pathMustExist},
		{"stored_modulesets.pb", pathMustExist},
		{"current_modules_init.pb", pathMustNotExist},
	} {
		if err := waitForPath(ctx, filepath.Join(chimeraPath, e.filename), e.cond); err != nil {
			return err
		}
	}

	if err := waitForApksOptimized(ctx, chimeraPath); err != nil {
		return err
	}

	targetTar := filepath.Join(outputDir, GMSCoreCacheArchive)

	testing.ContextLogf(ctx, "Compressing GMS Core caches to %q", targetTar)
	if err := testexec.CommandContext(ctx, "tar", "-cvpf", targetTar, "-C", gmsRootUnderHome, appChimera).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to compress GMS Core caches")
	}

	testing.ContextLogf(ctx, "Collecting stat results for files under %q", chimeraPath)

	// statResult holds data obtained by stat command.
	type statResult struct {
		path         string
		accessRights string // Obtained by "stat -c %a"
		numBlocks    string // Obtained by "stat -c %b"
	}
	var statResults []statResult
	filepath.Walk(chimeraPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Strip "/home/root/${USER_HASH}/android-data" prefix from the path.
		androidPath := strings.Replace(path, androidDataDir, "", 1)

		info, err = os.Lstat(path)
		if err != nil {
			return errors.Wrapf(err, "failed to stat %q", path)
		}
		if info.Mode()&os.ModeSymlink == os.ModeSymlink {
			// File is a symbolic link.
			realPath, err := os.Readlink(path)
			if err != nil {
				return errors.Wrapf(err, "failed to get real path for %q", path)
			}
			if !strings.HasPrefix(realPath, "/data/") {
				// File is a symbolic link to a file outside of /data/
				// and it is not exposed on CrOS file system.
				// Run stat command on Android file system via adb shell.
				out, err := a.Command(
					ctx, "stat", "-c", "%a:%b", realPath).Output(testexec.DumpLogOnError)
				statVals := strings.Split(strings.TrimSpace(string(out)), ":")
				if err != nil || len(statVals) != 2 {
					return errors.Wrapf(err, "failed to stat %q : %q", realPath, string(out))
				}
				statResults = append(statResults,
					statResult{path: androidPath, accessRights: statVals[0], numBlocks: statVals[1]})
				return nil
			}
		}

		// File is not a symbolic link, or a symbolic link to a file under /data/,
		// which is exposed on CrOS file system. Run stat command on CrOS.
		out, err := testexec.CommandContext(ctx, "stat", "-c", "%a:%b", path).Output()
		statVals := strings.Split(strings.TrimSpace(string(out)), ":")
		if err != nil || len(statVals) != 2 {
			return errors.Wrapf(err, "failed to stat %q : %q", path, string(out))
		}
		statResults = append(statResults,
			statResult{path: androidPath, accessRights: statVals[0], numBlocks: statVals[1]})
		return nil
	})

	testing.ContextLogf(ctx, "Generating %q from collected stat results", LayoutTxt)
	var layoutLines []string
	for _, r := range statResults {
		layoutLines = append(layoutLines, fmt.Sprintf("%s:%s:%s", r.path, r.accessRights, r.numBlocks))
	}
	sort.Strings(layoutLines)
	layout := strings.Join(layoutLines, "\n")
	if err := ioutil.WriteFile(filepath.Join(outputDir, LayoutTxt), []byte(layout), 0644); err != nil {
		return errors.Wrapf(err, "failed to generate %q for %q", LayoutTxt, chimeraPath)
	}

	// Packages cache
	src := filepath.Join(androidDataDir, packagesPath)
	packagesPathLocal := filepath.Join(outputDir, PackagesCacheXML)
	if err := fsutil.CopyFile(src, packagesPathLocal); err != nil {
		return err
	}

	// GSF cache
	src = filepath.Join(androidDataDir, gsfDatabase)
	dst := filepath.Join(outputDir, GSFCache)
	if err := fsutil.CopyFile(src, dst); err != nil {
		return err
	}

	// Extract GMS Core location and create manifest for this directory.
	b, err := ioutil.ReadFile(packagesPathLocal)
	if err != nil {
		return errors.Wrapf(err, "failed to read %q", packagesPathLocal)
	}

	gmsCorePath := regexp.MustCompile(`<package name=\"com\.google\.android\.gms\".+codePath=\"(\S+)\".+>`).FindStringSubmatch(string(b))
	if gmsCorePath == nil {
		return errors.Wrapf(err, "failed to parse %q", packagesPathLocal)
	}

	manifestPath := filepath.Join(outputDir, GMSCoreManifest)
	testing.ContextLogf(ctx, "Capturing GMS Core manifest for %q to %q", gmsCorePath[1], GMSCoreManifest)
	// stat -c "%n %s" "$0" gives name and file size in bytes
	// date +%s%N -r "$0" | cut -b1-13 gives modification time with millisecond resolution.
	// sha256sum -b "$0" gives sha256 check sum
	// tr \"\n\" \" \" to remove new line ending and have 3 commands outputs in one line.
	const perFileCmd = `stat -c "%n %s" "$0" | tr "\n" " "  && date +%s%N -r "$0" | cut -b1-13 | tr "\n" " "  && sha256sum -b "$0"`
	out, err := a.Command(
		ctx, "/system/bin/find", "-L", gmsCorePath[1], "-type", "f", "-exec", "sh", "-c", perFileCmd, "{}", ";").Output(testexec.DumpLogOnError)
	if err != nil {
		return errors.Wrapf(err, "failed to create GMS Core manifiest: %q", string(out))
	}

	if err := ioutil.WriteFile(manifestPath, out, 0644); err != nil {
		return errors.Wrapf(err, "failed to save GMS Core manifest to %q", manifestPath)
	}

	return nil
}

// waitForPath waits up to 1 minute or ctx deadline for the specified path to exist or not
// exist depending on pathCondition c.
func waitForPath(ctx context.Context, path string, c pathCondition) error {
	testing.ContextLogf(ctx, "Waiting for path %q", path)
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		_, err := os.Stat(path)
		if err != nil && !os.IsNotExist(err) {
			return testing.PollBreak(errors.Wrapf(err, "failed to stat %s", path))
		}
		exists := err == nil
		if c == pathMustExist {
			if !exists {
				return errors.Wrapf(err, "path %s still does not exist", path)
			}
		} else {
			if exists {
				return errors.Wrapf(err, "path %s still exists", path)
			}
		}
		return nil
	}, &testing.PollOptions{Timeout: time.Minute, Interval: time.Second}); err != nil {
		return errors.Wrapf(err, "failed to wait for path %s", path)
	}
	return nil
}

// waitForApksOptimized waits up to 1 minute or ctx deadline for all APKs in given root path are
// optimized, which means no *.flock locks and *.odex/*.vdex exist and matches actual APK count.
func waitForApksOptimized(ctx context.Context, root string) error {
	testing.ContextLogf(ctx, "Waiting for APKs optimized %q", root)
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// Calculate number of files per extension.
		perExtCnt := map[string]int{}
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				ext := filepath.Ext(info.Name())
				perExtCnt[ext] = perExtCnt[ext] + 1
			}
			return nil
		})
		if err != nil {
			return testing.PollBreak(errors.Wrapf(err, "failed to walk %q", root))
		}
		apkCnt := perExtCnt[".apk"]
		if perExtCnt[".flock"] != 0 {
			return errors.Wrapf(err, "file lock detected in %q", root)
		}
		if apkCnt == 0 {
			return testing.PollBreak(errors.Wrapf(err, "no APK found in %q", root))
		}
		vdexCnt := perExtCnt[".vdex"]
		odexCnt := perExtCnt[".odex"]
		if apkCnt != vdexCnt || apkCnt != odexCnt {
			return errors.Wrapf(err, "not everything yet optimized in %q. APK count: %d, vdex: %d, odex: %d", root, apkCnt, vdexCnt, odexCnt)
		}
		return nil
	}, &testing.PollOptions{Timeout: time.Minute, Interval: time.Second}); err != nil {
		return errors.Wrapf(err, "failed to wait for APKs optimized %s", root)
	}
	return nil
}
