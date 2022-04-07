// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Tool for generating a json file used in video_decode_accelerator_tests from
// a video file. The json file is created in the same directory as the video file.
// This script uses chromiumos/tast/errors, so it needs to run with
// ~/trunk/src/platform/tast/tools/go.sh.
//
// Usage example:
// $  ~/trunk/src/platform/tast/tools/go.sh run generate_json_for_decodertest.go test-25fps.h264
// $  ~/trunk/src/platform/tast/tools/go.sh run generate_json_for_decodertest.go resolution_change.av1.ivf
//
// Note that "num_fragments" in the json file is probably not correct for H264
// streams. The value needs to be obtained by running video_decode_accelerator_tests.
package main

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"chromiumos/tast/errors"
)

// JSONInfo stores the info in a json file used in video_decode_accelerator_tests.
type JSONInfo struct {
	File         string   `json:"file"`
	FileChecksum string   `json:"file_checksum"`
	Profile      string   `json:"profile"`
	BitDepth     int      `json:"bit_depth"`
	Width        int      `json:"width"`
	Height       int      `json:"height"`
	FrameRate    int      `json:"frame_rate"`
	NumFrames    int      `json:"num_frames"`
	NumFragments int      `json:"num_fragments"`
	MD5Checksums []string `json:"md5_checksums"`
}

// Profile represents a video codec profile.
type Profile int

const (
	h264Baseline Profile = iota
	h264Main
	h264High
	h264Extended
	vp8Any
	vp90
	vp92
	av1Main
	hevcMain
	hevcMainStillPicture
	hevcMain10
	hevcRext
)

// Codec represents a video codec.
type Codec int

const (
	h264 Codec = iota
	vp8
	vp9
	av1
	hevc
)

func profileToString(p Profile) string {
	switch p {
	case h264Baseline:
		return "H264PROFILE_BASELINE"
	case h264Main:
		return "H264PROFILE_MAIN"
	case h264High:
		return "H264PROFILE_HIGH"
	case h264Extended:
		return "H264PROFILE_EXTENDED"
	case vp8Any:
		return "VP8PROFILE_ANY"
	case vp90:
		return "VP9PROFILE_PROFILE0"
	case vp92:
		return "VP9PROFILE_PROFILE2"
	case av1Main:
		return "AV1PROFILE_PROFILE_MAIN"
	case hevcMain:
		return "HEVCPROFILE_MAIN"
	case hevcMainStillPicture:
		return "HEVCPROFILE_MAIN_STILL_PICTURE"
	case hevcMain10:
		return "HEVCPROFILE_MAIN10"
	case hevcRext:
		return "HEVCPROFILE_RANGE_EXTENSION"
	}
	return ""
}

func profileToCodec(p Profile) Codec {
	switch p {
	case h264Baseline, h264Main, h264High, h264Extended:
		return h264
	case vp8Any:
		return vp8
	case vp90, vp92:
		return vp9
	case av1Main:
		return av1
	case hevcMain, hevcMainStillPicture, hevcMain10, hevcRext:
		return hevc
	}
	return -1
}

func ffprobeCodecToProfile(codec, profile string) (Profile, error) {
	switch codec {
	case "h264":
		switch profile {
		case "Constrained Baseline":
			return h264Baseline, nil
		case "Baseline":
			return h264Baseline, nil
		case "Main":
			return h264Main, nil
		case "High":
			return h264High, nil
		case "Extended":
			return h264Extended, nil
		}
	case "vp8":
		return vp8Any, nil
	case "vp9":
		switch profile {
		case "Profile 0":
			return vp90, nil
		case "Profile 2":
			return vp92, nil
		}
	case "av1":
		switch profile {
		case "Main":
			return av1Main, nil
		}
	case "hevc":
		switch profile {
		case "Main":
			return hevcMain, nil
		case "Main Still Picture":
			return hevcMainStillPicture, nil
		case "Main 10":
			return hevcMain10, nil
		case "Rext":
			return hevcRext, nil
		}
	}

	return 0, errors.Errorf("unsupported codec=%s and profile=%s", codec, profile)
}

func parseStream(file string) (width, height, bitDepth, frameRate int, prof Profile, err error) {
	out, err := exec.Command("ffprobe", "-v", "quiet", "-print_format", "json",
		"-select_streams", "v", "-show_streams", file).Output()
	if err != nil {
		err = errors.Wrap(err, "failed executing ffprobe")
		return
	}

	type FFProbeJSON struct {
		Streams []struct {
			CodecName string `json:"codec_name"`
			Profile   string `json:"profile"`
			PixFmt    string `json:"pix_fmt"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
			FrameRate string `json:"r_frame_rate"`
		} `json:"streams"`
	}

	var jsonData FFProbeJSON
	if err = json.Unmarshal(out, &jsonData); err != nil {
		err = errors.Wrap(err, "failed unmarshaling json file")
		return
	}

	if len(jsonData.Streams) != 1 {
		err = errors.Errorf("ffprobe detects multiple or no video streams: %+v", jsonData.Streams)
		return
	}

	stream := jsonData.Streams[0]
	prof, err = ffprobeCodecToProfile(stream.CodecName, stream.Profile)
	if err != nil {
		return
	}

	switch stream.PixFmt {
	case "yuv420p", "yuv444p", "gray":
		bitDepth = 8
	case "yuv420p10le", "yuv422p10le", "yuv444p10le":
		bitDepth = 10
	case "gray12le", "yuv420p12le", "yuv422p12le", "yuv444p12le":
		bitDepth = 12
	default:
		err = errors.Errorf("unknown pix_fmt=%s", stream.PixFmt)
		return
	}

	fpsFrac := strings.Split(stream.FrameRate, "/")
	if len(fpsFrac) != 2 {
		err = errors.Errorf("invalid framerate=%s", stream.FrameRate)
		return
	}

	fpsNum, err := strconv.ParseFloat(fpsFrac[0], 64)
	if err != nil {
		return
	}

	fpsDenom, err := strconv.ParseFloat(fpsFrac[1], 64)
	if err != nil {
		return
	}

	frameRate = int(math.Round(fpsNum / fpsDenom))
	width = stream.Width
	height = stream.Height

	return
}

func genMD5FFMPEG(file string) ([]string, error) {
	out, err := exec.Command("ffmpeg", "-f", "framemd5", "-", "-i", file).Output()
	if err != nil {
		return []string{}, errors.Wrap(err, "failed executing ffmpeg")
	}

	// Example output format:
	// 0,          0,          0,        1,   115200, a5dad6170eb13fc5cbc6fe3511d44053
	// 0,          1,          1,        1,   115200, e056362baaf13dd0f888e67a681ab381
	// 0,          2,          2,        1,   115200, ee0c33d2b92e0443ca5770bd0c56911f
	var md5s []string
	for _, l := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if len(l) == 0 || l[0] == '#' {
			continue
		}

		data := strings.Split(l, ", ")
		if len(data) < 6 {
			return []string{}, errors.Errorf("unexpected format: %s", l)
		}

		md5 := strings.TrimSpace(data[5])
		if len(md5) != 32 {
			return []string{}, errors.Errorf("invalid md5 length: %s", md5)
		}

		md5s = append(md5s, md5)
	}
	return md5s, nil
}

func genMD5VPX(file, bin string) ([]string, error) {
	// -o option is necessary for libvpx to output each frame, and with --md5
	// the md5 of each frame is output but a frame file is not created.
	out, err := exec.Command(bin, "-o", "output%w_%h_%4.yuv", "--i420", "--md5", file).Output()
	if err != nil {
		return []string{}, errors.Wrapf(err, "failed executing %s", bin)
	}

	// Example output format:
	// a6ddd21f5f4e7424b6e7a1f2925fb33b  output320_240_0001.yuv
	// 41c77adcfd29abfaad62a057855adeaa  output320_240_0002.yuv
	// afdb44531614034e4a4a90c805a5d3b1  output320_240_0003.yuv
	var md5s []string
	for _, l := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if len(l) == 0 {
			continue
		}

		md5 := strings.Fields(l)[0]
		if len(md5) != 32 {
			return []string{}, errors.Errorf("invalid md5 length: %s", md5)
		}

		md5s = append(md5s, md5)
	}
	return md5s, nil
}

func genMD5(file string, profile Profile) ([]string, error) {
	switch profileToCodec(profile) {
	case h264, hevc:
		return genMD5FFMPEG(file)
	case vp8, vp9:
		return genMD5VPX(file, "vpxdec")
	case av1:
		return genMD5VPX(file, "aomdec")
	}

	return []string{}, errors.Errorf("unknown codec: %v", profileToString(profile))
}

func computeFileCheckSum(file string) (string, error) {
	f, err := os.Open(file)
	if err != nil {
		return "", errors.Wrap(err, "failed opening file")
	}
	defer f.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, f); err != nil {
		return "", errors.Wrap(err, "failed computing md5")
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

func genJSONInfo(file string) (JSONInfo, error) {
	var info JSONInfo
	var err error

	info.File = file
	if info.FileChecksum, err = computeFileCheckSum(file); err != nil {
		return info, err
	}

	var profile Profile
	if info.Width, info.Height, info.BitDepth, info.FrameRate, profile, err = parseStream(file); err != nil {
		return info, errors.Wrap(err, "failed parsing stream")
	}
	md5s, err := genMD5(file, profile)
	if err != nil {
		return info, errors.Wrap(err, "failed generating frame hashes")
	}

	info.Profile = profileToString(profile)
	info.NumFrames = len(md5s)
	info.MD5Checksums = md5s

	// This is probably not correct for H264 streams. However, the value cannot
	// be known by ffprobe. The value needs to be obtained by running
	// video_decode_accelerator_tests.
	info.NumFragments = len(md5s)

	return info, nil
}

func writeJSON(file string, info JSONInfo) (string, error) {
	s, err := json.MarshalIndent(info, "", "\t")
	if err != nil {
		return "", errors.Wrap(err, "failed marshaling json")
	}

	jsonPath := file + ".json"
	return jsonPath, ioutil.WriteFile(jsonPath, s, 0644)
}

func main() {
	var exitCode int
	for _, file := range os.Args[1:] {
		ext := filepath.Ext(file)
		if ext != ".ivf" && ext != ".h264" && ext != ".hevc" {
			fmt.Fprintf(os.Stderr, "The file's extension must be ivf, h264 or hevc: %s\n", file)
			exitCode = 1
			continue
		}

		info, err := genJSONInfo(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed getting json data for %s: %v\n", file, err)
			exitCode = 1
			continue
		}

		jsonFile, err := writeJSON(file, info)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed writing json data for %s: %v\n", file, err)
			exitCode = 1
			continue
		}

		fmt.Println("Created", jsonFile)
	}

	os.Exit(exitCode)
}
