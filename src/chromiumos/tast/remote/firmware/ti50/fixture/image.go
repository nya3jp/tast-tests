// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package fixture provides ti50 devboard related fixtures.
package fixture

import (
	"context"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

const (
	// BuildURL is the arg name for the directory of the gs build or full path of the image (local or in gs).
	BuildURL = "buildurl"

	// Ti50Image fixture downloads the ti50 image bin.
	Ti50Image = "ti50Image"

	// SystemTestAutoImage fixture downloads the system_test_auto image bin.
	SystemTestAutoImage = "systemTestAutoImage"

	// imageBin is the name of the image file, it is the same for both images.
	imageBin = "ti50_Unknown_PrePVT_ti50-accessory-nodelocked-ro-premp.bin"

	// branchImageBin is used instead of imageBin on branch builders.
	branchImageBin = "ti50_Unknown_PrePVT_ti50-accessory-mp.bin"

	gsPrefix = "gs://"

	imageDownloadTimeout = 30 * time.Second
	imageDeleteTimeout   = 5 * time.Second
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:            Ti50Image,
		Desc:            "Provides access to a Ti50 image",
		Contacts:        []string{"tast-fw-library-reviewers@google.com", "jbettis@google.com"},
		Impl:            &imageImpl{image: Ti50Image},
		Vars:            []string{BuildURL},
		SetUpTimeout:    imageDownloadTimeout,
		TearDownTimeout: imageDeleteTimeout,
	})
	testing.AddFixture(&testing.Fixture{
		Name:            SystemTestAutoImage,
		Desc:            "Uses devboardsvc to flash a system_test_auto image",
		Contacts:        []string{"tast-fw-library-reviewers@google.com", "jbettis@google.com"},
		Impl:            &imageImpl{image: SystemTestAutoImage},
		Vars:            []string{BuildURL},
		SetUpTimeout:    imageDownloadTimeout,
		TearDownTimeout: imageDeleteTimeout,
	})
}

// ImageValue provides access to a image binary.
type ImageValue struct {
	imagePath string
	imageType string
}

// ImagePath returns the path to the image binary.
func (v *ImageValue) ImagePath() string {
	return v.imagePath
}

// ImageType returns the type of the image binary.
func (v *ImageValue) ImageType() string {
	return v.imageType
}

type imageImpl struct {
	image      string
	downloaded bool
	v          *ImageValue
}

func (i *imageImpl) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	url, _ := s.Var(BuildURL)
	i.v = &ImageValue{imageType: i.image}

	if err := i.downloadImage(ctx, url); err != nil {
		s.Fatal("download image: ", err)
	}

	return i.v
}

func (i *imageImpl) Reset(ctx context.Context) error {
	return nil
}

func (i *imageImpl) PreTest(ctx context.Context, s *testing.FixtTestState) {
}

func (i *imageImpl) PostTest(ctx context.Context, s *testing.FixtTestState) {
}

func (i *imageImpl) TearDown(ctx context.Context, s *testing.FixtState) {
	if i.downloaded && i.v.imagePath != "" {
		if err := os.Remove(i.v.imagePath); err != nil {
			s.Errorf("Failed to remove downloaded image %q: %v", i.v.imagePath, err)
		}
	}
}

func (i *imageImpl) String() string {
	return i.image
}

// downloadImage downloads the image from google storage if necessary.
// inputURL can be a local file, a gs file, or a gs build folder.
func (i *imageImpl) downloadImage(ctx context.Context, inputURL string) error {
	if i.image == "" {
		return nil
	}

	if inputURL == "" {
		testing.ContextLogf(ctx, "-var=%s= not provided, assuming the devboard has a %s image", BuildURL, i)
		i.v.imagePath = ""
		return nil
	}

	var imageType string
	switch i.image {
	case Ti50Image:
		imageType = "ti50"
	case SystemTestAutoImage:
		imageType = "system_test_auto"
	default:
		return errors.Errorf("unknown image type: %q", i.image)
	}

	if inputURL[:len(gsPrefix)] == gsPrefix {
		fullURL := inputURL
		// Assume URL is a build folder if it doesn't end in .bin.
		if fullURL[len(fullURL)-4:] != ".bin" {
			// Assume branch builds have a -channel in the URL.
			var subDir string
			bin := branchImageBin
			if !strings.Contains(inputURL, "-channel/") {
				// Postsubmit builder images are 1 subdir deeper.
				subDir = imageType + ".tar.bz2"
				bin = imageBin
			}
			fullURL = gsPrefix + filepath.Join(inputURL[len(gsPrefix):], subDir, bin)
		}
		f, err := ioutil.TempFile("", imageType+"_")
		if err != nil {
			return errors.Wrap(err, "create temp image file")
		}
		f.Close()

		args := []string{"cp", fullURL, f.Name()}
		testing.ContextLogf(ctx, "Download image: gsutil %s", strings.Join(args, " "))
		cmd := exec.CommandContext(ctx, "gsutil", args...)
		if err := cmd.Run(); err != nil {
			return errors.Wrapf(err, "download %q", fullURL)
		}
		i.v.imagePath = f.Name()
		i.downloaded = true
		return nil
	}

	i.v.imagePath = inputURL
	return nil
}
