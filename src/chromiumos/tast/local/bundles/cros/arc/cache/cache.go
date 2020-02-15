// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cache

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

// PackagesCacheMode represents a flag that determines whether packages_cache.xml
// will be copied within ARC after boot.
type PackagesCacheMode int

const (
	// Copy forces to set caches and copy packages cache to the preserved destination.
	Copy PackagesCacheMode = iota
	// SkipCopy forces to ignore caches and copy packages cache to the preserved destination.
	SkipCopy
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
	// GmsCoreCacheArchive defines the GMS Core cache tar file name.
	GmsCoreCacheArchive = "gms_core_cache.tar"
	// GsfCache defines the GSF cache database file name.
	GsfCache = "gservices_cache.db"
)

// BootARCWithPackagesCacheMode boots ARC with caches turned on or off, depending on the mode parameter. In
// each case, resulting files (app_chimera tar archive, layout.txt) are copied to the specified output
// directory. On success, non-nil pointers are returned that must be closed by the calling function. However,
// if there is an error, both pointers will be nil.
func BootARCWithPackagesCacheMode(ctx context.Context, mode PackagesCacheMode, outputDir string) (cr *chrome.Chrome, a *arc.ARC, retErr error) {
	args := []string{"--arc-disable-app-sync", "--arc-disable=play-auto-install"}
	if mode == SkipCopy {
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

	a, err = arc.New(ctx, outputDir)
	if err != nil {
		return cr, nil, errors.Wrap(err, "could not start ARC")
	}
	defer func() {
		if retErr != nil && a != nil {
			a.Close()
		}
	}()

	const (
		gmsRoot      = "/data/user_de/0/com.google.android.gms"
		appChimera   = "app_chimera"
		tmpTarFile   = "/sdcard/Download/temp_gms_caches.tar"
		packagesPath = "/data/system/packages_copy.xml"
		gsfDatabase  = "/system/etc/gservices_cache/databases/gservices.db"
	)

	chimeraPath := filepath.Join(gmsRoot, appChimera)

	// waitForAndroidPath waits up to 1 minute or ctx deadline for the specified path to
	// exist or not exist depending on pathCondition c.
	waitForAndroidPath := func(ctx context.Context, a *arc.ARC, path string, c pathCondition) error {
		testing.ContextLogf(ctx, "Waiting for Android path %q", path)
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
			return nil, nil, err
		}
	}

	// app_chimera is now accessible via ADB in ARCVM but not in ARC++ container due to ARCVM has
	// userdebug build variation. Once switching to the user build this won't longer work. Use
	// agnostic way by archiving content to tar to the public space using bootstrap command
	// 'android-sh' which has enough permissions to do this.
	testing.ContextLogf(ctx, "Compressing GMS Core caches to %q", tmpTarFile)
	out, err := arc.BootstrapCommand(
		ctx, "/vendor/bin/tar", "-cvpf", tmpTarFile, "-C", gmsRoot, appChimera).Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "compression: %s failed: %q", chimeraPath, string(out))
	}
	// Cleanup temp tar.
	defer arc.BootstrapCommand(ctx, "/vendor/bin/rm", "-f", tmpTarFile).Run()

	// Pull archive to the host and unpack it.
	targetTar := filepath.Join(outputDir, GmsCoreCacheArchive)
	testing.ContextLogf(ctx, "Pulling GMS Core caches to %q", targetTar)
	if err = a.PullFile(ctx, tmpTarFile, targetTar); err != nil {
		return nil, nil, errors.Wrapf(err, "failed to pull %q from Android to %q", tmpTarFile, targetTar)
	}

	// Use BootstrapCommand to avoid permission limitation accessing chimera path via adb.
	layoutPath := filepath.Join(outputDir, LayoutTxt)
	testing.ContextLogf(ctx, "Capturing GMS Core caches layout to %q", layoutPath)
	out, err = arc.BootstrapCommand(
		ctx, "/vendor/bin/find", "-L", chimeraPath, "-exec", "stat", "-Lc", "%n:%a:%b", "{}", "+").Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to gather app_chimera file attributes: %q", string(out))
	}

	lines := strings.Split(string(out), "\n")
	sort.Strings(lines)
	layout := strings.Join(lines, "\n")
	err = ioutil.WriteFile(layoutPath, []byte(layout), 0644)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to generate %s for %s", LayoutTxt, chimeraPath)
	}

	// pullFileFromDevice pulls src file from Android to dst usign cat.
	pullFileFromDevice := func(src, dst string) error {
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

	// Packages cache
	if err := pullFileFromDevice(packagesPath, filepath.Join(outputDir, PackagesCacheXML)); err != nil {
		return nil, nil, err
	}

	// GSF cache
	if err := pullFileFromDevice(gsfDatabase, filepath.Join(outputDir, GsfCache)); err != nil {
		return nil, nil, err
	}

	return cr, a, nil
}
