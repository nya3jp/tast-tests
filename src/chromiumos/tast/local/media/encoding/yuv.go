// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package encoding

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
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/media/videotype"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// md5OfYUV is the MD5 value of the YUV file decoded by vpxdec.
// Since decoding algorithm is deterministic and the encoding is lossless, MD5 value of video raw data decoded by each webM should always be the same.
// These values are listed for the safety check to ensure we are always testing the same raw streams for result consistency.
var md5OfYUV = map[string]string{
	"bear-320x192.i420.yuv":    "14c9ac6f98573ab27a7ed28da8a909c0",
	"bear-320x192.nv12.yuv":    "8aedc0da37b7e6f15255375f57eb3241",
	"crowd-1920x1080.i420.yuv": "96f60dd6ff87ba8b129301a0f36efc58",
	"crowd-1920x1080.nv12.yuv": "0d1933e69f932519794586f81b133bb8",
	"tulip2-1280x720.i420.yuv": "1b95123232922fe0067869c74e19cd09",
	"tulip2-1280x720.nv12.yuv": "898a3e1bb3b8d2bdd137f92067d42106",
	"tulip2-640x360.i420.yuv":  "094bd827de18ca196a83cc6442b7b02f",
	"tulip2-640x360.nv12.yuv":  "750a9d254415858f821b8df06a5f3d48",
	"tulip2-320x180.i420.yuv":  "55be7124b3aec1b72bfb57f433297193",
	"tulip2-320x180.nv12.yuv":  "7899814a845a5342c6b4a6da7e494cc0",
	"vidyo1-1280x720.i420.yuv": "b8601dd181bb2921fffce3fbb896351e",
	"vidyo1-1280x720.nv12.yuv": "1c9bb2a27b76c35280412e7fa1b08fc2",
	"crowd-3840x2160.i420.yuv": "c0cf5576391ec6e2439a8d0fc7207662",
	"crowd-3840x2160.nv12.yuv": "9e0baa401565324a54c3dc67479b1470",
	"crowd-641x361.i420.yuv":   "124d3e29ea68eaba0dc35243b4dfc27b",
	"crowd-641x361.nv12.yuv":   "f2eabbe28eae5bfcf5f8aa0b50bf9119",
}

// PrepareYUV decodes webMFile and creates the associated YUV file for test whose pixel format is pixelFormat.
// The returned value is the path of the created YUV file. It must be removed in the end of test, because its size is expected to be large.
// The input WebM files are vp9 codec. They are generated from raw YUV data by libvpx like "vpxenc foo.yuv -o foo.webm --codec=vp9 -w <width> -h <height> --lossless=1"
// Please use "--lossless=1" option. Lossless compression is required to ensure we are testing streams at the same quality as original raw streams,
// to test encoder capabilities (performance, bitrate convergence, etc.) correctly and with sufficient complexity/PSNR.
func PrepareYUV(ctx context.Context, webMFile string, pixelFormat videotype.PixelFormat, size coords.Size) (string, error) {
	const webMSuffix = ".vp9.webm"
	if !strings.HasSuffix(webMFile, webMSuffix) {
		return "", errors.Errorf("source video %v must be VP9 WebM", webMFile)
	}
	webMName := filepath.Base(webMFile)

	yuvFile := strings.TrimSuffix(webMFile, ".vp9.webm")
	switch pixelFormat {
	case videotype.I420:
		yuvFile += ".i420.yuv"
	case videotype.NV12:
		yuvFile += ".nv12.yuv"
	}
	yuvName := filepath.Base(yuvFile)

	// If the raw video file already exists and the hash matches the expected value we can skip extraction.
	if _, err := os.Stat(yuvFile); !os.IsNotExist(err) {
		yuvHash, err := calculateHash(yuvFile)
		if err != nil {
			return "", err
		}

		if hash, found := md5OfYUV[yuvName]; found && yuvHash == hash {
			testing.ContextLogf(ctx, "Skipping extraction of %s: %s already exists", webMName, yuvName)
			return yuvFile, nil
		}
	}

	tf, err := os.Create(yuvFile)
	if err != nil {
		return "", errors.Wrap(err, "failed to create a temporary YUV file")
	}
	keep := false
	defer func() {
		tf.Close()
		if !keep {
			os.Remove(yuvFile)
		}
	}()

	testing.ContextLogf(ctx, "Executing vpxdec %s to prepare YUV data %s", webMName, yuvName)
	// TODO(hiroh): When YV12 test case is added, try generate YV12 yuv here by passing "--yv12" instead of "--i420".
	cmd := testexec.CommandContext(ctx, "vpxdec", webMFile, "-o", yuvFile, "--codec=vp9", "--i420")
	if err := cmd.Run(); err != nil {
		cmd.DumpLog(ctx)
		return "", errors.Wrap(err, "vpxdec failed")
	}

	// If pixelFormat is NV12, conversion from I420 to NV12 is performed.
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

		// Rename the temporary file to the yuv output file.
		tf.Close()
		cf.Close()
		if err := os.Rename(cf.Name(), yuvFile); err != nil {
			return "", errors.Wrap(err, "failed to rename YUV file")
		}
	}

	// This guarantees that the generated yuv file (i.e. input of VEA test) is the same on all platforms.
	yuvHash, err := calculateHash(yuvFile)
	if err != nil {
		return "", err
	}
	if yuvHash != md5OfYUV[yuvName] {
		return "", errors.Errorf("unexpected MD5 value of %s (got %s, want %s)", yuvName, yuvHash, md5OfYUV[yuvName])
	}

	keep = true
	return yuvFile, nil
}

// PrepareYUVFilesFromWebM encodes webMName and creates yuv file whose format is I420.
// Returns the paths of the YUV file and the associated JSON file used in the test.
func PrepareYUVFilesFromWebM(ctx context.Context, webMPath, srcYUVJSONPath string) (string, string, error) {
	yuvPath, err := PrepareYUV(ctx, webMPath, videotype.I420, coords.NewSize(0, 0) /* placeholder size */)
	if err != nil {
		return "", "", errors.Wrap(err, "failed to prepare YUVFile")
	}

	dstYUVJSONPath := yuvPath + ".json"
	if err := fsutil.CopyFile(srcYUVJSONPath, dstYUVJSONPath); err != nil {
		os.Remove(yuvPath)
		return "", "", errors.Wrapf(err, "failed to copy json file: %v %v", srcYUVJSONPath, dstYUVJSONPath)

	}
	return yuvPath, dstYUVJSONPath, nil
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
// Read data from r by either 32MB until EOF reached, perform the conversion on RAM
// and write the data to w.
func convertI420ToNV12(w io.Writer, r io.Reader, size coords.Size) error {
	yLen := size.Width * size.Height
	uvLen := size.Width * size.Height / 2
	planeLen := yLen + uvLen
	const maxBufferSize = 1048576 * 32 // 32MB
	bufSize := maxBufferSize / planeLen * planeLen
	buf := make([]byte, bufSize)
	uvBuf := make([]byte, uvLen)
	for {
		endOfFile := false
		readSize, err := r.Read(buf)
		if err == io.EOF {
			endOfFile = true
		} else if err != nil {
			return err
		}

		numPlanes := readSize / planeLen
		for i := 0; i < numPlanes; i++ {
			uLen := uvLen / 2
			uOffset := i*planeLen + yLen
			vOffset := uOffset + uLen
			for j := 0; j < uLen; j++ {
				uvBuf[2*j] = buf[uOffset+j]
				uvBuf[2*j+1] = buf[vOffset+j]
			}
			copy(buf[uOffset:uOffset+uvLen], uvBuf)
		}

		writeSize, err := w.Write(buf[:readSize])
		if err != nil {
			return nil
		} else if writeSize != readSize {
			return errors.Errorf("invalid writing size, got=%d, want=%d",
				readSize, writeSize)
		}

		if endOfFile {
			break
		}
	}
	return nil
}

// calculateHash calculates the MD5 hash of the specified file.
func calculateHash(filepath string) (string, error) {
	f, err := os.Open(filepath)
	if err != nil {
		return "", errors.Wrap(err, "failed to open YUV file")
	}
	defer f.Close()

	hasher := md5.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return "", errors.Wrap(err, "failed to read YUV file")
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}
