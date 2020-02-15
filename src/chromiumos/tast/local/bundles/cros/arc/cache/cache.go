// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cache

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"chromiumos/tast/errors"
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
		tmpTarFile   = "/sdcard/Download/temp_gms_caches.tar"
		packagesPath = "/data/system/packages_copy.xml"
		gsfDatabase  = "/system/etc/gservices_cache/databases/gservices.db"
	)

	chimeraPath := filepath.Join(gmsRoot, appChimera)
	for _, e := range []struct {
		filename string
		cond     pathCondition
	}{
		{"current_config.fb", pathMustExist},
		{"current_fileapks.pb", pathMustExist},
		{"stored_modulesets.pb", pathMustExist},
		{"current_modules_init.pb", pathMustNotExist},
	} {
		if err := waitForAndroidPath(ctx, a, filepath.Join(chimeraPath, e.filename), e.cond); err != nil {
			return err
		}
	}

	// Note, 'which find/tar/rm resolves /vendor/bin/*. BootstrapCommand requires to use absolute
	// paths. That is why /vendor/bin is used as a root for commands.

	// app_chimera is accessible via ADB in userdebug/eng but not in user builds.
	// Use agnostic way by archiving content to tar to the public space using bootstrap
	// command 'android-sh' which has enough permissions to do this.
	// TODO(b/148832630): get rid of BootstrapCommand.
	testing.ContextLogf(ctx, "Compressing GMS Core caches to %q", tmpTarFile)
	out, err := arc.BootstrapCommand(
		ctx, "/vendor/bin/tar", "-cvpf", tmpTarFile, "-C", gmsRoot, appChimera).Output(testexec.DumpLogOnError)
	// Cleanup temp tar in any case.
	defer arc.BootstrapCommand(ctx, "/vendor/bin/rm", "-f", tmpTarFile).Run()
	if err != nil {
		return errors.Wrapf(err, "compression: %s failed: %q", chimeraPath, string(out))
	}

	// Pull archive to the host and unpack it.
	targetTar := filepath.Join(outputDir, GMSCoreCacheArchive)
	testing.ContextLogf(ctx, "Pulling GMS Core caches to %q", targetTar)
	if err := a.PullFile(ctx, tmpTarFile, targetTar); err != nil {
		return errors.Wrapf(err, "failed to pull %q from Android to %q", tmpTarFile, targetTar)
	}

	// Use BootstrapCommand to avoid permission limitation accessing chimera path via adb.
	layoutPath := filepath.Join(outputDir, LayoutTxt)
	testing.ContextLogf(ctx, "Capturing GMS Core caches layout to %q", layoutPath)
	out, err = arc.BootstrapCommand(
		ctx, "/vendor/bin/find", "-L", chimeraPath, "-exec", "stat", "-Lc", "%n:%a:%b", "{}", "+").Output(testexec.DumpLogOnError)
	if err != nil {
		return errors.Wrapf(err, "failed to gather app_chimera file attributes: %q", string(out))
	}

	lines := strings.Split(string(out), "\n")
	sort.Strings(lines)
	layout := strings.Join(lines, "\n")
	if err := ioutil.WriteFile(layoutPath, []byte(layout), 0644); err != nil {
		return errors.Wrapf(err, "failed to generate %q for %q", LayoutTxt, chimeraPath)
	}

	// Packages cache
	packagesPathLocal := filepath.Join(outputDir, PackagesCacheXML)
	if err := pullARCFile(ctx, packagesPath, packagesPathLocal); err != nil {
		return err
	}

	// GSF cache
	if err := pullARCFile(ctx, gsfDatabase, filepath.Join(outputDir, GSFCache)); err != nil {
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
		ctx, "/vendor/bin/find", "-L", gmsCorePath[1], "-type", "f", "-exec", "sh", "-c", perFileCmd, "{}", ";").Output(testexec.DumpLogOnError)
	if err != nil {
		return errors.Wrapf(err, "failed to create GMS Core manifiest: %q", string(out))
	}

	if err := ioutil.WriteFile(manifestPath, out, 0644); err != nil {
		return errors.Wrapf(err, "failed to save GMS Core manifest to %q", manifestPath)
	}

	return nil
}

// pullARCFile pulls src file from Android to dst using cat. src is absolute Android path.
func pullARCFile(ctx context.Context, src, dst string) error {
	testing.ContextLogf(ctx, "Pulling %q to %q", src, dst)
	dstFile, err := os.Create(dst)
	if err != nil {
		return errors.Wrapf(err, "failed to create output: %q", dst)
	}
	defer dstFile.Close()

	// adb pull would fail due to permissions limitation. Use bootstrapped cat to copy it.
	cmd := arc.BootstrapCommand(ctx, "/bin/cat", src)
	cmd.Stdout = dstFile
	if err = cmd.Run(); err != nil {
		return errors.Wrapf(err, "failed to pull %q from Android", src)
	}
	return nil
}

// waitForAndroidPath waits up to 1 minute or ctx deadline for the specified path to exist or not
// exist depending on pathCondition c.
func waitForAndroidPath(ctx context.Context, a *arc.ARC, path string, c pathCondition) error {
	testing.ContextLogf(ctx, "Waiting for Android path %q", path)
	if err := testing.Poll(ctx, func(ctx context.Context) error {
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
	}, &testing.PollOptions{Timeout: time.Minute, Interval: time.Second}); err != nil {
		return errors.Wrapf(err, "failed to wait for path %s", path)
	}
	return nil
}
