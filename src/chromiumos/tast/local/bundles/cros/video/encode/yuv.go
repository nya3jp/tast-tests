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

// prepareYUV decodes webMFile and creates the associated YUV file for test whose pixel format is pixelFormat.
// The returned value is the path of the created YUV file. It must be removed in the end of test, because its size is expected to be large.
// The input WebM files are vp9 codec. They are generated from raw YUV data by libvpx like "vpxenc foo.yuv -o foo.webm --codec=vp9 --best -w 1920 -h 1080"
func prepareYUV(ctx context.Context, webMFile string, pixelFormat videotype.PixelFormat, size videotype.Size) (string, error) {
	webMName := filepath.Base(webMFile)
	yuvName := strings.TrimSuffix(webMName, ".vp9.webm") + ".yuv"

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

	var convErr error
	if err := (func() error {
		var out io.Writer = tf
		if pixelFormat == videotype.NV12 {
			conv := newI420ToNV12Converter(out, size)
			defer func() {
				convErr = conv.Close()
			}()
			out = conv
		}
		out = io.MultiWriter(hasher, out)

		testing.ContextLogf(ctx, "Executing vpxdec %s to prepare YUV data %s", webMName, yuvName)
		// TODO(hiroh): When YV12 test case is added, try generate YV12 yuv here by passing "--yv12" instead of "--i420".
		cmd := testexec.CommandContext(ctx, "vpxdec", webMFile, "--codec=vp9", "--i420", "-o", "-")
		cmd.Stdout = out
		if err := cmd.Run(); err != nil {
			cmd.DumpLog(ctx)
			return errors.Wrap(err, "vpxdec failed")
		}
		return nil
	})(); err != nil {
		return "", err
	} else if convErr != nil {
		return "", convErr
	}

	// This guarantees that the generated yuv file (i.e. input of VEA test) is the same on all platforms.
	hash := hex.EncodeToString(hasher.Sum(nil))
	if hash != md5OfYUV[yuvName] {
		return "", errors.Errorf("unexpected MD5 value of %s (got %s, want %s)", yuvName, hash, md5OfYUV[yuvName])
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

type i420ToNV12Converter struct {
	pw   *io.PipeWriter
	done chan error
}

func newI420ToNV12Converter(w io.Writer, size videotype.Size) io.WriteCloser {
	pr, pw := io.Pipe()
	ch := make(chan error)
	c := &i420ToNV12Converter{pw, ch}
	go func() {
		ch <- convertI420ToNV12(w, pr, size)
	}()
	return c
}

func (c *i420ToNV12Converter) Write(p []byte) (int, error) {
	return c.pw.Write(p)
}

func (c *i420ToNV12Converter) Close() error {
	cerr := c.pw.Close()
	werr := <-c.done
	if cerr != nil {
		return cerr
	}
	return werr
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
