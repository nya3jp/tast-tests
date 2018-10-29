// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package encode

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/video/lib/videotype"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// YuvToWebm denotes the webm file that is to be decoded for acquiring the yuv file.
// The webm files are vp9 codec. They are generated from raw yuv data by libvpx like "vpxenc foo.yuv -o foo.webm --codec=vp9 --best -w 1920 -h 1080"
var YuvToWebm = map[string]string{
	"crowd-1920x1080.yuv": "crowd-1920x1080.webm",
	"tulip2-1280x720.yuv": "tulip2-1280x720.webm",
	"tulip2-640x360.yuv":  "tulip2-640x360.webm",
	"tulip2-320x180.yuv":  "tulip2-320x180.webm",
}

// md5OfYuv is the md5sum of the yuv file decoded by vpxdec.
var md5OfYuv = map[string]string{
	"crowd-1920x1080.yuv": "3e1b2da6ba437289c305d92a742912fb",
	"tulip2-1280x720.yuv": "709f016edc9a1b70ba23716eb6e87aa2",
	"tulip2-640x360.yuv":  "66f2aa4b2225008cafcfcd19f74a125d",
	"tulip2-320x180.yuv":  "83f682fb225c17532b8345b4a926f4b7",
}

// PrepareYuv decodes webmFile and creates the associated yuv file for test whose pixel format is format.
// The returned value is the path of the created yuv file. It must be removed in the end of test, because its size is expected to be large.
func PrepareYuv(ctx context.Context, webmFile string, format videotype.PixelFormat) (string, error) {
	// TODO(hiroh): Support non-I420 pixel format by using "convert" command after vpxenc.
	// NOTE: "imagemagick" is already installed on test image and thus "convert" command is already available on test image.
	if format != videotype.I420 {
		return "", errors.New("prepare yuv file by dynamically is only available for I420")
	}

	webmName := filepath.Base(webmFile)
	yuvFile, err := createYuvFile(webmName)
	if err != nil {
		return "", errors.Wrapf(err, "failed to create temporary yuv file")
	}

	cmd := testexec.CommandContext(ctx, "vpxdec", webmFile, "--codec=vp9", "--i420", "-o", yuvFile)
	testing.ContextLogf(ctx, "Executing vpxdec %s to prepare yuv data %s", webmName, yuvFile)
	if err := cmd.Run(); err != nil {
		os.Remove(yuvFile)
		return "", errors.Wrap(err, "vpxdec failed")
	}

	md5sum, err := getMD5OfFile(yuvFile)
	if err != nil {
		os.Remove(yuvFile)
		return "", err
	}
	yuvName := webmToYuv(webmName)
	if md5sum != md5OfYuv[yuvName] {
		os.Remove(yuvFile)
		return "", errors.Errorf("unexpected md5sum of %s: %s (expected: %s)", yuvName, md5sum, md5OfYuv[yuvName])
	}
	return yuvFile, nil
}

// createYuvFile creates a temporary file for yuv data.
func createYuvFile(webmName string) (string, error) {
	f := webmToYuv(webmName)
	tf, err := ioutil.TempFile("", f)
	if err != nil {
		return "", err
	}
	if err := tf.Chmod(0666); err != nil {
		os.Remove(tf.Name())
		return "", err
	}

	return tf.Name(), nil

}

// webmToYuv gets yuv file name from webm file name looking YuvToWebm.
func webmToYuv(webm string) string {
	var f string
	for k, v := range YuvToWebm {
		if v == webm {
			f = k
		}
	}
	if len(f) == 0 {
		panic(fmt.Sprintf("not found yuv for ", webm))
	}
	return f
}

// GetMD5OfFile computes the md5 value of file.
func getMD5OfFile(path string) (string, error) {
	// Check md5 of yuv data.
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
