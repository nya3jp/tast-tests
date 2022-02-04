// Copyright 2022 The Chromium OS Authors. All rights reserved.
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
	// BuildURL is the arg name for the gs directory of the build or a local directory containing the imageBin.
	BuildURL = "buildurl"

	// Ti50Image fixture downloads the ti50 image bin.
	Ti50Image = "ti50Image"

	// SystemTestAutoImage fixture downloads the system_test_auto image bin.
	SystemTestAutoImage = "systemTestAutoImage"

	// imageBin is the name of the image file, it is the same for both images.
	imageBin = "ti50_Unknown_PrePVT_ti50-accessory-nodelocked-ro-premp.bin"

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
func (i *imageImpl) downloadImage(ctx context.Context, gsOrDir string) error {
	if i.image == "" {
		return nil
	}

	if gsOrDir == "" {
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

	if gsOrDir[:len(gsPrefix)] == gsPrefix {
		fullURL := gsPrefix + filepath.Join(gsOrDir[len(gsPrefix):], imageType, imageBin)

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

	i.v.imagePath = filepath.Join(gsOrDir, imageType, imageBin)
	return nil
}
