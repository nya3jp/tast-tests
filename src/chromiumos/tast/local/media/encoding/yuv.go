// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package encoding

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/media/videotype"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

// md5OfYUV is the MD5 value of the YUV file decoded by vpxdec.
// Since the decoding algorithm is deterministic, the raw data MD5 value should always be the same.
// These values are listed for the safety check to ensure we are always testing the same raw streams for result consistency.
var md5OfYUV = map[string]string{
	"bear-320x192.i420.yuv":             "14c9ac6f98573ab27a7ed28da8a909c0",
	"bear-320x192.nv12.yuv":             "8aedc0da37b7e6f15255375f57eb3241",
	"crowd-1920x1080.i420.yuv":          "96f60dd6ff87ba8b129301a0f36efc58",
	"crowd-1920x1080.nv12.yuv":          "0d1933e69f932519794586f81b133bb8",
	"gipsrestat-1280x720.i420.yuv":      "acc6bb983c198c8db5ffc5d5699cb235",
	"gipsrestat-640x360.i420.yuv":       "a92466c51d8626f263771ba16f7d5d02",
	"gipsrestat-320x180.i420.yuv":       "556d908527aea47e0e02440bf6c35861",
	"tulip2-1280x720.i420.yuv":          "1b95123232922fe0067869c74e19cd09",
	"tulip2-1280x720.nv12.yuv":          "898a3e1bb3b8d2bdd137f92067d42106",
	"tulip2-960x540.i420.yuv":           "c0ff5b6c62ba8914aa073d630acd6309",
	"tulip2-640x360.i420.yuv":           "094bd827de18ca196a83cc6442b7b02f",
	"tulip2-640x360.nv12.yuv":           "750a9d254415858f821b8df06a5f3d48",
	"tulip2-480x270.i420.yuv":           "06e0ac8b028a78ed4cd8dda5ab5bceec",
	"tulip2-320x180.i420.yuv":           "55be7124b3aec1b72bfb57f433297193",
	"tulip2-320x180.nv12.yuv":           "7899814a845a5342c6b4a6da7e494cc0",
	"vidyo1-1280x720.i420.yuv":          "b8601dd181bb2921fffce3fbb896351e",
	"vidyo1-1280x720.nv12.yuv":          "1c9bb2a27b76c35280412e7fa1b08fc2",
	"crowd-3840x2160.i420.yuv":          "c0cf5576391ec6e2439a8d0fc7207662",
	"crowd-3840x2160.nv12.yuv":          "9e0baa401565324a54c3dc67479b1470",
	"crowd-641x361.i420.yuv":            "124d3e29ea68eaba0dc35243b4dfc27b",
	"crowd-641x361.nv12.yuv":            "f2eabbe28eae5bfcf5f8aa0b50bf9119",
	"crowd-320x180_30frames.i420.yuv":   "795d9e03fc4631245558cc522462a1e5",
	"crowd-640x360_30frames.i420.yuv":   "134fecaaae471820dede6c761e4d8f4b",
	"crowd-960x540_30frames.i420.yuv":   "c1ab2a4af9bc76fc5d659fcf19fbee09",
	"crowd-1280x720_30frames.i420.yuv":  "f26bff398809056165be970922492281",
	"crowd-1920x1080_30frames.i420.yuv": "13e4f50ad665e27c2a8603d6e65a0a39",
	"crowd-3840x2160_30frames.i420.yuv": "a739d49d4072bc91ca7b9b223e4b117f",
}

// PrepareYUV decodes webMFile and creates the associated YUV file for test whose pixel format is pixelFormat.
// The returned value is the path of the created YUV file. It must be removed in the end of test, because its size is expected to be large.
// The input WebM files are vp9 codec. They are generated from raw YUV data by libvpx like "vpxenc foo.yuv -o foo.webm --codec=vp9 -w <width> -h <height> --lossless=1"
// Please use "--lossless=1" option. Lossless compression is required to ensure we are testing streams at the same quality as original raw streams,
// to test encoder capabilities (performance, bitrate convergence, etc.) correctly and with sufficient complexity/PSNR.
// TODO(b/177856221): Removes the functionality of producing NV12 format, so that this always produces an I420 file.
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

	threads := runtime.NumCPU()
	if threads > 16 {
		// The maximum number of threads is the same as chrome.
		// https://source.chromium.org/chromium/chromium/src/+/main:media/base/limits.h;l=83;drc=5539ecff898c79b0771340051d62bf81649e448d
		threads = 16
	}
	// TODO(hiroh): When YV12 test case is added, try generate YV12 yuv here by passing "--yv12" instead of "--i420".
	command := []string{"vpxdec", webMFile, "-t", strconv.Itoa(threads), "-o", yuvFile, "--codec=vp9", "--i420"}
	testing.ContextLogf(ctx, "Running %s", shutil.EscapeSlice(command))
	cmd := testexec.CommandContext(ctx, command[0], command[1:]...)
	if err := cmd.Run(); err != nil {
		cmd.DumpLog(ctx)
		return "", errors.Wrap(err, "vpxdec failed")
	}

	// If pixelFormat is NV12, conversion from I420 to NV12 is performed.
	if pixelFormat == videotype.NV12 {
		cf, err := CreatePublicTempFile(yuvName)
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

// PrepareYUVJSON creates a json file for yuvPath by copying jsonPath.
// The first return value is the path of the created JSON file.
func PrepareYUVJSON(ctx context.Context, yuvPath, jsonPath string) (string, error) {
	yuvJSONPath := yuvPath + ".json"
	if err := fsutil.CopyFile(jsonPath, yuvJSONPath); err != nil {
		return "", errors.Wrapf(err, "failed to copy json file: %v %v", jsonPath, yuvJSONPath)

	}
	return yuvJSONPath, nil
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
