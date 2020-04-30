// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"

	"github.com/golang/protobuf/ptypes/empty"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/arc/cache"
	"chromiumos/tast/local/testexec"
	arcpb "chromiumos/tast/services/cros/arc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			arcpb.RegisterCacheValidationServiceServer(srv, &CacheValidationService{s: s})
		},
	})
}

// CacheValidationService implements tast.cros.arc.CacheValidationService.
type CacheValidationService struct {
	s *testing.ServiceState
}

func (c *CacheValidationService) GetJarURL(ctx context.Context, args *arcpb.Args) (*arcpb.Url, error) {
	const (
		// Base path
		buildsRoot = "gs://chromeos-arc-images/builds"

		// Name of jar file
		jarName = "org.chromium.arc.cachebuilder.jar"
	)
	// Detect buildID
	var propertyFile string
	if len(args.ExtraArgs) > 1 && args.ExtraArgs[1] == "--enable-arcvm" {
		propertyFile = "/usr/share/arcvm/properties/build.prop"
	} else {
		propertyFile = "/usr/share/arc/properties/build.prop"
	}

	cmd := exec.Command("cat", propertyFile)
	buildProp, err := cmd.Output()
	if err != nil {
		return nil, errors.Wrap(err, "failed to read ARC build property file remotely")
	}

	buildPropStr := string(buildProp)

	mBuildID := regexp.MustCompile(`(\n|^)ro.build.version.incremental=(.+)(\n|$)`).FindStringSubmatch(buildPropStr)
	if mBuildID == nil {
		return nil, errors.Errorf("ro.build.version.incremental is not found in %q", buildPropStr)
	}

	urlStr := fmt.Sprintf("%s/%s/%s/%s", buildsRoot, args.ExtraArgs[0], mBuildID[2], jarName)

	url := &arcpb.Url{
		Url: urlStr,
	}
	return url, nil
}

func (c *CacheValidationService) GetResult(ctx context.Context, args *arcpb.Args) (*arcpb.Result, error) {
	td, err := ioutil.TempDir("", "")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create a temp dir")
	}
	// defer os.RemoveAll(td)

	withCacheDir := filepath.Join(td, "with_cache")
	withoutCacheDir := filepath.Join(td, "without_cache")

	if err := os.Mkdir(withCacheDir, 0755); err != nil {
		return nil, errors.Wrapf(err, "could not make output subdirectory: %s", withCacheDir)
	}
	if err := os.Mkdir(withoutCacheDir, 0755); err != nil {
		return nil, errors.Wrapf(err, "could not make output subdirectory: %s", withoutCacheDir)
	}

	// Boot ARC with and without caches enabled, and copy relevant files to output directory.
	// s.Log("Starting ARC, with packages cache disabled")
	cr, a, err := cache.OpenSession(ctx, cache.PackagesSkipCopy, cache.GMSCoreDisabled, args.ExtraArgs, withoutCacheDir)
	if err != nil {
		return nil, errors.Wrap(err, "Booting ARC failed")
	}
	defer cr.Close(ctx)
	defer a.Close()

	if err := cache.CopyCaches(ctx, a, withoutCacheDir); err != nil {
		return nil, errors.Wrap(err, "Copying caches failed")
	}

	// s.Log("Starting ARC, with packages cache enabled")
	cr, a, err = cache.OpenSession(ctx, cache.PackagesCopy, cache.GMSCoreEnabled, args.ExtraArgs, withCacheDir)
	if err != nil {
		return nil, errors.Wrap(err, "Booting ARC failed")
	}
	defer cr.Close(ctx)
	defer a.Close()

	if err := cache.CopyCaches(ctx, a, withCacheDir); err != nil {
		return nil, errors.Wrap(err, "Copying caches failed")
	}

	// unpackGmsCoreCaches unpack GMS core caches which are returned by cache.BootARC packed
	// in tar.
	unpackGmsCoreCaches := func(outputDirs []string) error {
		for _, outputDir := range outputDirs {
			tarPath := filepath.Join(outputDir, cache.GMSCoreCacheArchive)
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
		return nil, errors.Wrap(err, "could not prepare GMS Core caches: ")
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

	if err := saveOutput(filepath.Join(td, "app_chimera.diff"),
		testexec.CommandContext(ctx, "diff", "--recursive", "--no-dereference",
			filepath.Join(withCacheDir, "app_chimera"),
			filepath.Join(withoutCacheDir, "app_chimera"))); err != nil {
		return nil, errors.Wrap(err, "Error validating app_chimera folders: ")
	}
	if err := saveOutput(filepath.Join(td, "layout.diff"),
		testexec.CommandContext(ctx, "diff",
			filepath.Join(withCacheDir, cache.LayoutTxt),
			filepath.Join(withoutCacheDir, cache.LayoutTxt))); err != nil {
		return nil, errors.Wrap(err, "Error validating app_chimera layouts: ")
	}

	// packagesWithCache := filepath.Join(td, "packages_with_cache.xml")
	// if err := a.PushFile(ctx, filepath.Join(withCacheDir, cache.PackagesCacheXML), packagesWithCache); err != nil {
	// 	return nil, errors.Wrapf(err, "Could not push %s to Android: ", packagesWithCache)
	// }
	// packagesWithoutCache := filepath.Join(td, "packages_without_cache.xml")
	// if err := a.PushFile(ctx, filepath.Join(withoutCacheDir, cache.PackagesCacheXML), packagesWithoutCache); err != nil {
	// 	return nil, errors.Wrapf(err, "Could not push %s to Android: ", packagesWithoutCache)
	// }
	generatedPackagesCache := filepath.Join(td, cache.PackagesCacheXML)
	if err := a.PullFile(ctx, filepath.Join("/system/etc", cache.PackagesCacheXML), generatedPackagesCache); err != nil {
		return nil, errors.Wrapf(err, "could not pull %s from Android, this may mean that pre-generated packages cache was not installed when building the image: ", generatedPackagesCache)
	}

	packagesWithCache := filepath.Join(withCacheDir, cache.PackagesCacheXML)
	packagesWithoutCache := filepath.Join(withoutCacheDir, cache.PackagesCacheXML)
	// generatedPackagesCache := filepath.Join("/system/etc", cache.PackagesCacheXML)

	res := &arcpb.Result{
		TempDir:                td,
		PackagesWithCache:      packagesWithCache,
		PackagesWithoutCache:   packagesWithoutCache,
		GeneratedPackagesCache: generatedPackagesCache,
	}
	return res, nil
}

func (c *CacheValidationService) RemoveTempFiles(ctx context.Context, td *arcpb.TempDir) (*empty.Empty, error) {
	if err := os.RemoveAll(td.TempDir); err != nil {
		return nil, errors.Wrap(err, "failed to remove temp files")
	}
	return &empty.Empty{}, nil
}
