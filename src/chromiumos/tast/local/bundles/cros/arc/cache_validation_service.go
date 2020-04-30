// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/golang/protobuf/ptypes/empty"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
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

// generatePackages boots ARC with or without caches enabled, and copy relevant files to output directory.
func generatePackages(ctx context.Context, packagesDir string, a *arc.ARC) (string, error) {
	if err := cache.CopyCaches(ctx, a, packagesDir); err != nil {
		return "", errors.Wrap(err, "Copying caches failed")
	}

	tarPath := filepath.Join(packagesDir, cache.GMSCoreCacheArchive)
	if err := testexec.CommandContext(
		ctx, "tar", "-xvpf", tarPath, "-C", packagesDir).Run(); err != nil {
		return "", errors.Wrapf(err, "decompression %q failed", tarPath)
	}
	if err := os.Remove(tarPath); err != nil {
		return "", errors.Wrapf(err, "failed to cleanup %q", tarPath)
	}

	return filepath.Join(packagesDir, cache.PackagesCacheXML), nil
}

func (c *CacheValidationService) Generate(ctx context.Context, args *arcpb.Args) (*arcpb.CacheValidationResponse, error) {
	td, err := ioutil.TempDir("", "")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create a temp dir")
	}
	// Host calls RemoveTempFiles to remove this directory after the test completes.
	// defer os.RemoveAll(td)

	withCacheDir := filepath.Join(td, "with_cache")
	withoutCacheDir := filepath.Join(td, "without_cache")

	if err := os.Mkdir(withCacheDir, 0755); err != nil {
		return nil, errors.Wrapf(err, "could not make output subdirectory: %s", withCacheDir)
	}
	if err := os.Mkdir(withoutCacheDir, 0755); err != nil {
		return nil, errors.Wrapf(err, "could not make output subdirectory: %s", withoutCacheDir)
	}

	cr, a, err := cache.OpenSession(ctx, cache.PackagesCopy, cache.GMSCoreEnabled, args.ExtraArgs, withCacheDir)
	if err != nil {
		return nil, errors.Wrap(err, "Booting ARC failed")
	}
	defer cr.Close(ctx)
	defer a.Close()

	withCache, err := generatePackages(ctx, withCacheDir, a)
	if err != nil {
		return nil, errors.Wrapf(err, "could not generate packages for %s :", withCacheDir)
	}

	cr, a, err = cache.OpenSession(ctx, cache.PackagesSkipCopy, cache.GMSCoreDisabled, args.ExtraArgs, withoutCacheDir)
	if err != nil {
		return nil, errors.Wrap(err, "Booting ARC failed")
	}
	defer cr.Close(ctx)
	defer a.Close()

	withoutCache, err := generatePackages(ctx, withoutCacheDir, a)
	if err != nil {
		return nil, errors.Wrapf(err, "could not generate packages for %s :", withoutCacheDir)
	}

	generatedPackagesCache := filepath.Join(td, cache.PackagesCacheXML)
	if err := a.PullFile(ctx, filepath.Join("/system/etc", cache.PackagesCacheXML), generatedPackagesCache); err != nil {
		return nil, errors.Wrapf(err, "could not pull %s from Android, this may mean that pre-generated packages cache was not installed when building the image: ", generatedPackagesCache)
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

	res := &arcpb.CacheValidationResponse{
		TempDir:                td,
		PackagesWithCache:      withCache,
		PackagesWithoutCache:   withoutCache,
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
