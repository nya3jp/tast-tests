// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package encode

import (
	"bytes"
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
	yuvName := webMToYUV(webMName)
	testing.ContextLogf(ctx, "Executing vpxdec %s to prepare YUV data %s", webMName, yuvName)
	// TODO(hiroh): When YV12 test case is added, try generate YV12 yuv here by passing "--yv12" instead of "--i420".
	cmd := testexec.CommandContext(ctx, "vpxdec", webMFile, "--codec=vp9", "--i420", "-o", "-")
	out, err := cmd.Output()
	if err != nil {
		cmd.DumpLog(ctx)
		return "", errors.Wrap(err, "vpxdec failed")
	}

	// This guarantees that the generated yuv file (i.e. input of VEA test) is the same on all platforms.
	md5Sum := md5.Sum(out)
	hexMD5Sum := hex.EncodeToString(md5Sum[:])
	if hexMD5Sum != md5OfYUV[yuvName] {
		return "", errors.Errorf("unexpected MD5 value of %s (got %s, want %s)", yuvName, hexMD5Sum, md5OfYUV[yuvName])
	}

	// If pixelFormat is NV12, conversion from I420 to NV12 is performed.
	// TODO(hiroh): Think about using libyuv by cgo to reduce the effort if we need to support more formats conversion.
	if pixelFormat == videotype.NV12 {
		out, err = convertI420ToNV12(out, size)
		if err != nil {
			return "", errors.Wrapf(err, "failed to convert I420 to NV12")
		}
	}

	return createYUVFile(yuvName, out)
}

// createYUVFile creates a temporary file for YUV data.
func createYUVFile(yuvName string, content []byte) (string, error) {
	f, err := ioutil.TempFile("", yuvName)
	if err != nil {
		return "", errors.Wrap(err, "failed to create temporary YUV file")
	}
	defer func() {
		if f == nil {
			return
		}
		f.Close()
		os.Remove(f.Name())
	}()

	if err := f.Chmod(0644); err != nil {
		return "", errors.Wrap(err, "failed to set temporary YUV file permission")
	}

	if _, err := f.Write(content); err != nil {
		return "", errors.Wrap(err, "failed to write YUV file content")
	}

	if err := f.Close(); err != nil {
		return "", errors.Wrap(err, "failed to close temporary YUV file")
	}

	name := f.Name()
	f = nil // Cancel clean up in defer.
	return name, nil
}

func webMToYUV(w string) string {
	return strings.TrimSuffix(w, ".vp9.webm") + ".yuv"
}

// convertI420ToNV12 fills NV12 YUV data in nv12, converting from i420 YUV in i420.
func convertI420ToNV12(i420 []byte, size videotype.Size) ([]byte, error) {
	frameSize := size.W * size.H * 3 / 2
	if len(i420)%frameSize != 0 {
		return nil, errors.Errorf("i420 size %d not multiple of frame size %d", len(i420), frameSize)
	}
	numFrames := len(i420) / frameSize

	r := bytes.NewReader(i420)
	var w bytes.Buffer
	yLen := size.W * size.H
	uvLen := size.W * size.H / 2
	uvBuf := make([]byte, uvLen)
	for i := 0; i < numFrames; i++ {
		// Write Y Plane as-is.
		if _, err := io.CopyN(&w, r, int64(yLen)); err != nil {
			return nil, err
		}
		if _, err := io.ReadFull(r, uvBuf); err != nil {
			return nil, err
		}
		// U and V Planes are interleaved.
		vOffset := uvLen / 2
		for j := 0; j < uvLen/2; j++ {
			if _, err := w.Write(uvBuf[j : j+1]); err != nil {
				return nil, err
			}
			if _, err := w.Write(uvBuf[vOffset+j : vOffset+j+1]); err != nil {
				return nil, err
			}
		}
	}
	return w.Bytes(), nil
}
