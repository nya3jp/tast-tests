// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cache

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/cryptohome"
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

	// Cryptohome dir for the current user. (OpenSession signs in as chrome.DefaultUser.)
	rootCryptDir, err := cryptohome.SystemPath(chrome.DefaultUser)
	if err != nil {
		return errors.Wrap(err, "failed getting the cryptohome directory for the user")
	}

	// android-data dir under the cryptohome dir (/home/root/${USER_HASH}/android-data)
	androidDataDir := filepath.Join(rootCryptDir, "android-data")

	gmsRootUnderHome := filepath.Join(androidDataDir, gmsRoot)
	chimeraPath := filepath.Join(gmsRootUnderHome, appChimera)
	for _, e := range []struct {
		filename string
		cond     pathCondition
	}{
		{"current_config.fb", pathMustExist},
		{"current_fileapks.pb", pathMustExist},
		{"stored_modulesets.pb", pathMustExist},
		{"current_modules_init.pb", pathMustNotExist},
	} {
		if err := waitForPath(ctx, filepath.Join(chimeraPath, e.filename), e.cond); err != nil {
			return err
		}
	}

	targetTar := filepath.Join(outputDir, GMSCoreCacheArchive)

	testing.ContextLogf(ctx, "Compressing GMS Core caches to %q", targetTar)
	cmd := testexec.CommandContext(
		ctx, "/bin/tar", "-cvpf", targetTar, "-C", gmsRootUnderHome, appChimera)
	if err := cmd.Run(); err != nil {
		cmd.DumpLog(ctx)
		return errors.Wrap(err, "failed to compress GMS Core caches")
	}

	layoutPath := filepath.Join(outputDir, LayoutTxt)
	testing.ContextLogf(ctx, "Capturing GMS Core caches layout to %q", layoutPath)
	out, err := testexec.CommandContext(
		ctx, "/usr/bin/find", "-L", chimeraPath, "-exec", "stat", "-c", "%n:%a:%b:%N", "{}", "+").Output()
	if err != nil {
		return errors.Wrapf(err, "failed to gather app_chimera file attributes: %q", string(out))
	}

	symbolicLinkPattern := regexp.MustCompile(`'.+' -> '(/system/.+)'`)

	// Process stat results.
	var lines []string
	for _, l := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		statVals := strings.Split(l, ":")
		if len(statVals) != 4 {
			return errors.Wrapf(err, "failed to process stat output: %q", l)
		}
		filepath := strings.Replace(statVals[0], androidDataDir, "", 1)
		linkInfo := statVals[3]

		m := symbolicLinkPattern.FindStringSubmatch(linkInfo)
		if m == nil {
			// File is not a symbolic link.
			// Output the stat result as is. (filepath:permission:blockSize)
			lines = append(lines, fmt.Sprintf("%s:%s:%s", filepath, statVals[1], statVals[2]))
		} else {
			// File is a symbolic link to a file under /system/, which is not exposed under
			// /home/root/${USER_HASH}/android-data/.
			// Use android-sh for obtaining stat info of the link target.
			linkTarget := string(m[1])
			out, err := arc.BootstrapCommand(
				ctx, "/vendor/bin/stat", "-Lc", "%n:%a:%b", linkTarget).Output(testexec.DumpLogOnError)
			arcStatVals := strings.Split(strings.TrimSpace(string(out)), ":")
			if err != nil || len(arcStatVals) != 3 {
				return errors.Wrapf(err, "failed to gather app_chimera file attributes: %q", string(out))
			}
			// Output the stat result obtained by android-sh -c stat. (filepath:permission:blockSize)
			lines = append(lines, fmt.Sprintf("%s:%s:%s", filepath, arcStatVals[1], arcStatVals[2]))
		}
	}

	sort.Strings(lines)
	layout := strings.Join(lines, "\n")
	if err := ioutil.WriteFile(layoutPath, []byte(layout), 0644); err != nil {
		return errors.Wrapf(err, "failed to generate %q for %q", LayoutTxt, chimeraPath)
	}

	// Packages cache
	src := filepath.Join(androidDataDir, packagesPath)
	packagesPathLocal := filepath.Join(outputDir, PackagesCacheXML)
	if err := testexec.CommandContext(ctx, "/bin/cp", src, packagesPathLocal).Run(); err != nil {
		return err
	}

	// GSF cache
	src = filepath.Join(androidDataDir, gsfDatabase)
	dst := filepath.Join(outputDir, GSFCache)
	if err := testexec.CommandContext(ctx, "/bin/cp", src, dst).Run(); err != nil {
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

	// Use BootstrapCommand to avoid permission limitation accessing chimera path via adb.
	manifestPath := filepath.Join(outputDir, GMSCoreManifest)
	testing.ContextLogf(ctx, "Capturing GMS Core manifest for %q to %q", gmsCorePath[1], GMSCoreManifest)
	// stat -c "%n %s" "$0" gives name and file size in bytes
	// date +%s%N -r "$0" | cut -b1-13 gives modification time with millisecond resolution.
	// sha256sum -b "$0" gives sha256 check sum
	// tr \"\n\" \" \" to remove new line ending and have 3 commands outputs in one line.
	const perFileCmd = `stat -c "%n %s" "$0" | tr "\n" " "  && date +%s%N -r "$0" | cut -b1-13 | tr "\n" " "  && sha256sum -b "$0"`
	out, err = arc.BootstrapCommand(
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
		var comm *testexec.Cmd
		var msg string
		if c == pathMustExist {
			comm = testexec.CommandContext(ctx, "test", "-e", path)
			msg = "does not exist"
		} else {
			comm = testexec.CommandContext(ctx, "test", "!", "-e", path)
			msg = "exists"
		}
		if err := comm.Run(); err != nil {
			return errors.Wrapf(err, "path %s still %s", path, msg)
		}
		return nil
	}, &testing.PollOptions{Timeout: time.Minute, Interval: time.Second}); err != nil {
		return errors.Wrapf(err, "failed to wait for path %s", path)
	}
	return nil
}
