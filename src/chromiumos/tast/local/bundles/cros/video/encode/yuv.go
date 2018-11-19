// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package encode

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/video/lib/videotype"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// md5OfYUV is the MD5 value of the YUV file decoded by vpxdec.
// Since decoding algorithm is deterministic, MD5 value of video raw data decoded by each webM should always be the same.
// These values are listed for the safety check to avoid flakieness.
var md5OfYUV = map[string]string{
	"bear-320x192.yuv":    "35e6307dbe8f92952ae0e8e3979dce02",
	"crowd-1920x1080.yuv": "3e1b2da6ba437289c305d92a742912fb",
	"tulip2-1280x720.yuv": "709f016edc9a1b70ba23716eb6e87aa2",
	"tulip2-640x360.yuv":  "66f2aa4b2225008cafcfcd19f74a125d",
	"tulip2-320x180.yuv":  "83f682fb225c17532b8345b4a926f4b7",
	// TODO(hiroh): Add md5sum for NV12.
}

// PrepareYUV decodes webMFile and creates the associated YUV file for test whose pixel format is pixelFormat.
// The returned value is the path of the created YUV file. It must be removed in the end of test, because its size is expected to be large.
// The input WebM files are vp9 codec. They are generated from raw YUV data by libvpx like "vpxenc foo.yuv -o foo.webm --codec=vp9 --best -w 1920 -h 1080"
func PrepareYUV(ctx context.Context, webMFile string, pixelFormat videotype.PixelFormat, w, h int) (string, error) {
	webMName := filepath.Base(webMFile)
	yuvFile, err := createYUVFile(webMName)
	if err != nil {
		return "", errors.Wrapf(err, "failed to create temporary YUV file")
	}

	// TODO(hiroh): Support YV12 format by passing "--yv12" instead of "--i420".
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

	yuvName := webMToYUV(webMName)
	if md5sum != md5OfYUV[yuvName] {
		os.Remove(yuvFile)
		return "", errors.Errorf("unexpected MD5 value of %s (got %s, want %s)", yuvName, md5sum, md5OfYUV[yuvName])
	}

	// If pixelFormat is NV12, conversion from I420 to NV12 is performed.
	if pixelFormat == videotype.NV12 {
		nv12File, err := createNV12FileFromI420File(ctx, yuvFile, w, h)
		if err != nil {
			os.Remove(yuvFile)
			return "", errors.Wrapf(err, "failed to convert I420 to NV12")
		}
		// Remove I420 yuv file, because it is no longer used.
		os.Remove(yuvFile)

		yuvFile = nv12File
	}

	return yuvFile, nil
}

// createYUVFile creates a temporary file for YUV data.
func createYUVFile(webMName string) (string, error) {
	f := webMToYUV(webMName)
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

func webMToYUV(w string) string {
	return strings.TrimSuffix(w, ".webm") + ".yuv"
}

// convertI420ToNV12 converts i420 YUV to one NV12 YUV.
func convertI420ToNV12(ctx context.Context, i420 []byte, w, h int, numFrames int) []byte {
	yLeng := w * h
	uvLeng := w * h / 2
	nv12 := make([]byte, (yLeng+uvLeng)*numFrames)
	testing.ContextLogf(ctx, "numFrames=%d", numFrames)
	for i := 0; i < numFrames; i++ {
		testing.ContextLogf(ctx, "i=%d", i)
		offset := (yLeng + uvLeng) * i
		// Write Y Plane as-is.
		copy(nv12[offset:offset+yLeng], nv12[offset:offset+yLeng])

		// U and V Planes are interleaved.
		uOffset := offset + yLeng
		vOffset := offset + yLeng + yLeng/4
		for j, uvItr := 0, offset+yLeng; j < w*h/4; j++ {
			nv12[uvItr] = i420[uOffset+j]
			uvItr++
			nv12[uvItr] = i420[vOffset+j]
			uvItr++
		}
	}
	return nv12
}

// createNV12FileFromI420File creates NV12 YUV file from I420 YUV file.
func createNV12FileFromI420File(ctx context.Context, i420File string, w, h int) (string, error) {
	nv12File, err := ioutil.TempFile("", strings.TrimSuffix(filepath.Base(i420File), filepath.Ext(i420File))+".nv12.yuv")
	if err != nil {
		return "", err
	}
	defer nv12File.Close()

	// Get
	info, err := os.Stat(i420File)
	if err != nil {
		os.Remove(nv12File.Name())
		return "", err
	}

	frameSize := int64(w * h * 3 / 2)
	i420FileLeng := info.Size()
	if i420FileLeng%frameSize != 0 {
		os.Remove(nv12File.Name())
		return "", errors.Errorf("%s size (=%d)cannot be divided by %d(=%dx%dx3/2)", i420FileLeng, frameSize, w, h)

	}
	numFrames := int(info.Size() / frameSize)

	i420, err := ioutil.ReadFile(i420File)
	if err != nil {
		os.Remove(nv12File.Name())
		return "", err
	}

	n, err := nv12File.Write(convertI420ToNV12(ctx, i420, w, h, numFrames))
	if err != nil {
		os.Remove(nv12File.Name())
		return "", err
	}
	if int64(n) != i420FileLeng {
		os.Remove(nv12File.Name())
		return "", errors.Errorf("failed to write UV plane data (%d bytes written, expected %d bytes)", n, w*h*3/2)
	}

	if err := nv12File.Chmod(0644); err != nil {
		os.Remove(nv12File.Name())
		return "", err
	}

	return nv12File.Name(), nil
}
