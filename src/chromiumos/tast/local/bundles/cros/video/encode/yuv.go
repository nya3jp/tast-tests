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

// webMToYUV denotes the YUV file that is associated with WebM file.
// The WebM files are vp9 codec. They are generated from raw YUV data by libvpx like "vpxenc foo.yuv -o foo.webm --codec=vp9 --best -w 1920 -h 1080"
var webMToYUV = map[string]string{
	"crowd-1920x1080.webm": "crowd-1920x1080.yuv",
	"tulip2-1280x720.webm": "tulip2-1280x720.yuv",
	"tulip2-640x360.webm":  "tulip2-640x360.yuv",
	"tulip2-320x180.webm":  "tulip2-320x180.yuv",
}

// md5OfYUV is the MD5 value of the YUV file decoded by vpxdec.
var md5OfYUV = map[string]string{
	"crowd-1920x1080.yuv": "3e1b2da6ba437289c305d92a742912fb",
	"tulip2-1280x720.yuv": "709f016edc9a1b70ba23716eb6e87aa2",
	"tulip2-640x360.yuv":  "66f2aa4b2225008cafcfcd19f74a125d",
	"tulip2-320x180.yuv":  "83f682fb225c17532b8345b4a926f4b7",
}

func init() {
	for _, v := range webMToYUV {
		if _, found := md5OfYUV[v]; !found {
			panic(fmt.Sprintf("No MD5 value for %s", v))
		}
	}
}

// PrepareYUV decodes webMFile and creates the associated YUV file for test whose pixel format is format.
// The returned value is the path of the created YUV file. It must be removed in the end of test, because its size is expected to be large.
func PrepareYUV(ctx context.Context, webMFile string, format videotype.PixelFormat) (string, error) {
	// TODO(hiroh): Support non-I420 pixel format by using "convert" command after vpxenc. Then, please let tast-local-tests-cros ebuild depend media-gfx/imagemagick.
	// NOTE: "imagemagick" is already installed on test image and thus "convert" command is already available on test image.
	if format != videotype.I420 {
		return "", errors.New("dynamic YUV files only available for I420")
	}

	webMName := filepath.Base(webMFile)
	yuvFile, err := createYUVFile(webMName)
	if err != nil {
		return "", errors.Wrapf(err, "failed to create temporary YUV file")
	}

	cmd := testexec.CommandContext(ctx, "vpxdec", webMFile, "--codec=vp9", "--i420", "-o", yuvFile)
	testing.ContextLogf(ctx, "Executing vpxdec %s to prepare YUV data %s", webMName, yuvFile)
	if err := cmd.Run(); err != nil {
		cmd.DumpLog(ctx)
		os.Remove(yuvFile)
		return "", errors.Wrap(err, "vpxdec failed")
	}

	// This guarantees that the generated yuv file (i.e. input of VEA test) is the same on all platforms.
	md5sum, err := getMD5OfFile(yuvFile)
	if err != nil {
		os.Remove(yuvFile)
		return "", err
	}
	yuvName := webMToYUV[webMName]
	if md5sum != md5OfYUV[yuvName] {
		os.Remove(yuvFile)
		return "", errors.Errorf("unexpected MD5 value of %s: %s (expected: %s)", yuvName, md5sum, md5OfYUV[yuvName])
	}
	return yuvFile, nil
}

// createYUVFile creates a temporary file for YUV data.
func createYUVFile(webMName string) (string, error) {
	f := webMToYUV[webMName]
	tf, err := ioutil.TempFile("", f)
	if err != nil {
		return "", err
	}
	if err := tf.Chmod(0644); err != nil {
		os.Remove(tf.Name())
		return "", err
	}
	return tf.Name(), nil

}

// getMD5OfFile computes the MD5 value of file.
func getMD5OfFile(path string) (string, error) {
	// Check MD5 of YUV data.
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
