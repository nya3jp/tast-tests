// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package webcodecs

import (
	"encoding/binary"
	"io"
	"os"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/media/encoding"
	"chromiumos/tast/local/media/videotype"
)

// writeIVFFileHeader and writeIVFFrameHeader writes IVF file header and frame header into bitstreamFile, respectively.
// See https://wiki.multimedia.cx/index.php/IVF.
func writeIVFFileHeader(bitstreamFile io.Writer, codec videotype.Codec, w, h, framerate, numFrames int) {
	bitstreamFile.Write([]byte{'D', 'K', 'I', 'F'})
	binary.Write(bitstreamFile, binary.LittleEndian, uint16(0))
	binary.Write(bitstreamFile, binary.LittleEndian, uint16(32))
	switch codec {
	case videotype.VP8:
		bitstreamFile.Write([]byte{'V', 'P', '8', '0'})
	case videotype.VP9:
		bitstreamFile.Write([]byte{'V', 'P', '9', '0'})
	default:
		panic("Unknown codec")
	}

	binary.Write(bitstreamFile, binary.LittleEndian, uint16(w))
	binary.Write(bitstreamFile, binary.LittleEndian, uint16(h))
	binary.Write(bitstreamFile, binary.LittleEndian, uint32(framerate))
	binary.Write(bitstreamFile, binary.LittleEndian, uint32(1))
	binary.Write(bitstreamFile, binary.LittleEndian, uint32(numFrames))
	binary.Write(bitstreamFile, binary.LittleEndian, uint32(0))
}

func writeIVFFrameHeader(bitstreamFile io.Writer, size uint32, timestamp uint64) {
	binary.Write(bitstreamFile, binary.LittleEndian, size)
	binary.Write(bitstreamFile, binary.LittleEndian, timestamp)
}

// saveBitstream saves bitstreams in a temporary directory. The saved format is H.264 Annex B format if codec is H264 and IVF file format otherwise.
// A file path of the saved bitstream file is returned. A caller has a responsibility to remove the file.
func saveBitstream(bitstreams [][]byte, codec videotype.Codec, width, height, framerate int) (string, error) {
	return saveTemporalLayerBitstream(bitstreams, codec, width, height, framerate, 0, []int{})
}

// saveTemporalLayerBitstream saves bitstreams in a temporary directory. The saved format is H.264 Annex B format if codec is H264 and IVF file format otherwise.
// If tids is non empty, the saved bitstream represents frames that are in temporal layers up to tid.
// A file path of the saved bitstream file is returned. A caller has a responsibility to remove the file.
func saveTemporalLayerBitstream(bitstreams [][]byte, codec videotype.Codec, width, height, framerate, temporalID int, temporalIDs []int) (createdFilePath string, err error) {
	var filePrefix string
	switch codec {
	case videotype.H264:
		filePrefix = "webcodecs.h264"
	case videotype.VP8, videotype.VP9:
		filePrefix = "webcodecs.ivf"
	}

	bitstreamFile, err := encoding.CreatePublicTempFile(filePrefix)
	if err != nil {
		return "", errors.Wrap(err, "failed creating temporary file")
	}
	defer bitstreamFile.Close()
	keep := false
	defer func() {
		if !keep {
			os.Remove(bitstreamFile.Name())
		}
	}()

	// Add Create IVF header
	if codec != videotype.H264 {
		writeIVFFileHeader(bitstreamFile, codec, width, height, framerate, len(bitstreams))
	}

	for i, b := range bitstreams {
		if len(temporalIDs) > 0 && temporalIDs[i] > temporalID {
			// Skip the encoded chunk b if it belongs to a temporal layer higher than temporalId.
			continue
		}

		if codec != videotype.H264 {
			timestamp := uint64(i * 1000 / framerate)
			writeIVFFrameHeader(bitstreamFile, uint32(len(b)), timestamp)
		}
		if writeSize, err := bitstreamFile.Write(b); err != nil {
			return "", err
		} else if writeSize != len(b) {
			return "", errors.Errorf("invalid writing size, got=%d, want=%d", writeSize, len(b))
		}
	}

	keep = true
	createdFilePath = bitstreamFile.Name()
	return
}
