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
	"chromiumos/tast/local/media/videotype"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// md5OfYUV is the MD5 value of the YUV file decoded by vpxdec.
// Since decoding algorithm is deterministic and the encoding is lossless, MD5 value of video raw data decoded by each webM should always be the same.
// These values are listed for the safety check to ensure we are always testing the same raw streams for result consistency.
var md5OfYUV = map[string]string{
	"bear-320x192.yuv":    "14c9ac6f98573ab27a7ed28da8a909c0",
	"crowd-1920x1080.yuv": "96f60dd6ff87ba8b129301a0f36efc58",
	"tulip2-1280x720.yuv": "1b95123232922fe0067869c74e19cd09",
	"tulip2-640x360.yuv":  "094bd827de18ca196a83cc6442b7b02f",
	"tulip2-320x180.yuv":  "55be7124b3aec1b72bfb57f433297193",
	"vidyo1-1280x720.yuv": "b8601dd181bb2921fffce3fbb896351e",
	"crowd-3840x2160.yuv": "c0cf5576391ec6e2439a8d0fc7207662",
	// TODO(hiroh): Add md5sum for NV12.
}

// PrepareYUV decodes webMFile and creates the associated YUV file for test whose pixel format is pixelFormat.
// The returned value is the path of the created YUV file. It must be removed in the end of test, because its size is expected to be large.
// The input WebM files are vp9 codec. They are generated from raw YUV data by libvpx like "vpxenc foo.yuv -o foo.webm --codec=vp9 -w <width> -h <height> --lossless=1"
// Please use "--lossless=1" option. Lossless compression is required to ensure we are testing streams at the same quality as original raw streams,
// to test encoder capabilities (performance, bitrate convergence, etc.) correctly and with sufficient complexity/PSNR.
func PrepareYUV(ctx context.Context, webMFile string, pixelFormat videotype.PixelFormat, size videotype.Size) (string, error) {
	const webMSuffix = ".vp9.webm"
	if !strings.HasSuffix(webMFile, webMSuffix) {
		return "", errors.Errorf("source video %v must be VP9 WebM", webMFile)
	}
	webMName := filepath.Base(webMFile)
	yuvName := strings.TrimSuffix(webMName, webMSuffix) + ".yuv"

	tf, err := publicTempFile(yuvName)
	if err != nil {
		return "", errors.Wrap(err, "failed to create a temporary YUV file")
	}
	keep := false
	defer func() {
		tf.Close()
		if !keep {
			os.Remove(tf.Name())
		}
	}()

	hasher := md5.New()
	out := io.MultiWriter(hasher, tf)

	testing.ContextLogf(ctx, "Executing vpxdec %s to prepare YUV data %s", webMName, yuvName)
	// TODO(hiroh): When YV12 test case is added, try generate YV12 yuv here by passing "--yv12" instead of "--i420".
	cmd := testexec.CommandContext(ctx, "vpxdec", webMFile, "--codec=vp9", "--i420", "-o", "-")
	cmd.Stdout = out
	if err := cmd.Run(); err != nil {
		cmd.DumpLog(ctx)
		return "", errors.Wrap(err, "vpxdec failed")
	}

	// This guarantees that the generated yuv file (i.e. input of VEA test) is the same on all platforms.
	hash := hex.EncodeToString(hasher.Sum(nil))
	if hash != md5OfYUV[yuvName] {
		return "", errors.Errorf("unexpected MD5 value of %s (got %s, want %s)", yuvName, hash, md5OfYUV[yuvName])
	}

	// If pixelFormat is NV12, conversion from I420 to NV12 is performed.
	// TODO(hiroh): Think about using libyuv by cgo to reduce the effort if we need to support more formats conversion.
	if pixelFormat == videotype.NV12 {
		cf, err := publicTempFile(yuvName)
		if err != nil {
			return "", errors.Wrap(err, "failed to create a temporary YUV file")
		}
		defer func() {
			cf.Close()
			os.Remove(cf.Name())
		}()

		if _, err := tf.Seek(0, io.SeekStart); err != nil {
			return "", err
		}

		if err := convertI420ToNV12(cf, tf, size); err != nil {
			return "", errors.Wrap(err, "failed to convert I420 to NV12")
		}

		// Make tf point to the converted file.
		tf, cf = cf, tf
	}

	keep = true
	return tf.Name(), nil
}

// publicTempFile creates a world-readable temporary file.
func publicTempFile(prefix string) (*os.File, error) {
	f, err := ioutil.TempFile("", prefix)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create a public temporary file")
	}
	if err := f.Chmod(0644); err != nil {
		f.Close()
		os.Remove(f.Name())
		return nil, errors.Wrap(err, "failed to create a public temporary file")
	}
	return f, nil
}

// convertI420ToNV12 converts i420 YUV to NV12 YUV.
func convertI420ToNV12(w io.Writer, r io.Reader, size videotype.Size) error {
	yLen := size.W * size.H
	uvLen := size.W * size.H / 2
	uvBuf := make([]byte, uvLen)
	for {
		// Write Y Plane as-is.
		if sz, err := io.CopyN(w, r, int64(yLen)); err != nil {
			if sz == 0 && err == io.EOF {
				break
			}
			return err
		}
		if _, err := io.ReadFull(r, uvBuf); err != nil {
			return err
		}
		// U and V Planes are interleaved.
		vOffset := uvLen / 2
		for j := 0; j < uvLen/2; j++ {
			if _, err := w.Write(uvBuf[j : j+1]); err != nil {
				return err
			}
			if _, err := w.Write(uvBuf[vOffset+j : vOffset+j+1]); err != nil {
				return err
			}
		}
	}
	return nil
}
