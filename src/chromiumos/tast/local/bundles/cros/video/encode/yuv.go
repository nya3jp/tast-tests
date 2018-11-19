// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package encode

import (
	"bufio"
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

// prepareYUV decodes webMFile and creates the associated YUV file for test whose pixel format is pixelFormat.
// The returned value is the path of the created YUV file. It must be removed in the end of test, because its size is expected to be large.
// The input WebM files are vp9 codec. They are generated from raw YUV data by libvpx like "vpxenc foo.yuv -o foo.webm --codec=vp9 --best -w 1920 -h 1080"
func prepareYUV(ctx context.Context, webMFile string, pixelFormat videotype.PixelFormat, size videotype.Size) (string, error) {
	webMName := filepath.Base(webMFile)
	yuvFile, err := createYUVFile(webMName)
	if err != nil {
		return "", errors.Wrapf(err, "failed to create temporary YUV file")
	}

	// TODO(hiroh): When YV12 test case is added, try generate YV12 yuv here by passing "--yv12" instead of "--i420".
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
	// TODO(hiroh): Think about using libyuv by cgo to reduce the effort if we need to support more formats conversion.
	if pixelFormat == videotype.NV12 {
		nv12File, err := createNV12FileFromI420File(yuvFile, size)
		// Remove I420 yuv file, because it is no longer used.
		os.Remove(yuvFile)
		if err != nil {
			return "", errors.Wrapf(err, "failed to convert I420 to NV12")
		}
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
	return strings.TrimSuffix(w, ".vp9.webm") + ".yuv"
}

// createNV12FileFromI420File creates NV12 YUV file from I420 YUV file.
// TODO(hiroh): Add unittest for this function.
func createNV12FileFromI420File(i420Path string, size videotype.Size) (string, error) {
	i420File, err := os.Open(i420Path)
	if err != nil {
		return "", err
	}
	defer i420File.Close()

	info, err := i420File.Stat()
	if err != nil {
		return "", err
	}

	frameSize := int64(size.W * size.H * 3 / 2)
	i420FileLen := info.Size()
	if i420FileLen%frameSize != 0 {
		return "", errors.Errorf("%s size %d not multiple of frame size %d", i420Path, i420FileLen, frameSize)
	}
	numFrames := int(info.Size() / frameSize)

	nv12File, err := ioutil.TempFile("", strings.TrimSuffix(filepath.Base(i420Path), filepath.Ext(i420Path))+".nv12.yuv")
	if err != nil {
		return "", err
	}
	toUnlink := nv12File.Name()
	defer func() {
		nv12File.Close()
		if toUnlink != "" {
			os.Remove(toUnlink)
		}
	}()

	if err := convertI420ToNV12(i420File, nv12File, size, numFrames); err != nil {
		return "", err
	}

	if err := nv12File.Chmod(0644); err != nil {
		return "", err
	}
	// Make toUnlink an empty string in order to not remove nv12File on success.
	toUnlink = ""
	return nv12File.Name(), nil
}

// convertI420ToNV12 fills NV12 YUV data in nv12, converting from i420 YUV in i420.
func convertI420ToNV12(i420 io.Reader, nv12 io.Writer, size videotype.Size, numFrames int) error {
	bw := bufio.NewWriter(nv12)
	yLen := size.W * size.H
	uvLen := size.W * size.H / 2
	uvBuf := make([]byte, uvLen)
	for i := 0; i < numFrames; i++ {
		// Write Y Plane as-is.
		if _, err := io.CopyN(bw, i420, int64(yLen)); err != nil {
			return err
		}
		if _, err := io.ReadFull(i420, uvBuf); err != nil {
			return err
		}
		// U and V Planes are interleaved.
		vOffset := uvLen / 2
		for j := 0; j < uvLen/2; j++ {
			bw.Write(uvBuf[j : j+1])
			bw.Write(uvBuf[vOffset+j : vOffset+j+1])
		}
	}
	return bw.Flush()
}
