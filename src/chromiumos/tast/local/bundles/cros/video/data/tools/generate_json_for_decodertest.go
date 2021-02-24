// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Tool for generating a json file used in video_decode_accelerator_tests from
// a video file. The json file is created in the same directory in the video file.
// This script uses chromiumos/tast/errors, so that it needs to run with
// ~/trunk/src/platform/tast/tools/go.sh.
//
// Usage example:
// $  ~/trunk/src/platform/tast/tools/go.sh run generate_json_for_decoder_test.go test-25fps.h264
// $  ~/trunk/src/platform/tast/tools/go.sh run generate_json_for_decoder_test.go resolution_change.av1.ivf
package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"chromiumos/tast/errors"
)

// JSONInfo stores the info in a json file used in video_decode_accelerator_tests.
type JSONInfo struct {
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
	vp8Any
	vp90
	vp92
	av1Main
)

// Codec represents a video codec.
type Codec int

const (
	h264 Codec = iota
	vp8
	vp9
	av1
)

func profileToString(p Profile) string {
	switch p {
	case h264Baseline:
		return "H264PROFILE_BASELINE"
	case h264Main:
		return "H264PROFILE_MAIN"
	case h264High:
		return "H264PROFILE_HIGH"
	case vp8Any:
		return "VP8PROFILE_ANY"
	case vp90:
		return "VP9PROFILE_PROFILE0"
	case vp92:
		return "VP9PROFILE_PROFILE2"
	case av1Main:
		return "AV1PROFILE_PROFILE_MAIN"
	}
	return ""
}

func profileToCodec(p Profile) Codec {
	switch p {
	case h264Baseline, h264Main, h264High:
		return h264
	case vp8Any:
		return vp8
	case vp90, vp92:
		return vp9
	case av1Main:
		return av1
	}
	return h264
}

func ffprobeCodecToProfile(codec, profile string) (Profile, error) {
	match := true
	p := h264Baseline
	switch codec {
	case "h264":
		switch profile {
		case "Baseline":
			p = h264Baseline
		case "Main":
			p = h264Main
		case "High":
			p = h264High
		default:
			match = false
		}
	case "vp8":
		p = vp8Any
	case "vp9":
		switch profile {
		case "Profile 0":
			p = vp90
		case "Profile 2":
			p = vp92
		default:
			match = false
		}
	case "av1":
		switch profile {
		case "Main":
			p = av1Main
		default:
			match = false
		}
	default:
		match = false
	}
	if !match {
		return p, errors.Errorf("unsupported codec=%s and profile=%s", codec, profile)
	}
	return p, nil
}

func parseStream(file string) (int, int, Profile, int, error) {
	const ffprobCmd = "ffprobe -v quiet -print_format json -select_streams v -show_streams"

	cmd := append(strings.Split(ffprobCmd, " "), file)
	out, err := exec.Command(cmd[0], cmd[1:]...).Output()
	if err != nil {
		return -1, -1, h264Baseline, -1, errors.Wrap(err, "failed executing ffprobe: ")
	}

	type FFProbeJSON struct {
		Streams []struct {
			CodecName string `json:"codec_name"`
			Profile   string `json:"profile"`
			PixFmt    string `json:"pix_fmt"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
		} `json:"streams"`
	}

	var jsonData FFProbeJSON
	if err = json.Unmarshal(out, &jsonData); err != nil {
		return -1, -1, h264Baseline, -1, errors.Wrap(err, "failed unmarshaling json file")
	}

	if len(jsonData.Streams) != 1 {
		return -1, -1, h264Baseline, -1, errors.New("ffprobe detects multiple video streams")
	}

	stream := jsonData.Streams[0]
	p, err := ffprobeCodecToProfile(stream.CodecName, stream.Profile)
	if err != nil {
		return -1, -1, h264Baseline, -1, err
	}

	bitDepth := 8
	if stream.PixFmt != "yuv420p" {
		if stream.PixFmt == "yuv420p10le" {
			bitDepth = 10
		} else {
			return -1, -1, h264Baseline, -1, errors.Errorf("unknown pix_fmt=%s", stream.PixFmt)
		}
	}

	return stream.Width, stream.Height, p, bitDepth, nil
}

func genMD5H264(file string) ([]string, error) {
	const decoderCmd = "ffmpeg -f framemd5 - -i"
	cmd := append(strings.Split(decoderCmd, " "), file)
	out, err := exec.Command(cmd[0], cmd[1:]...).Output()
	if err != nil {
		return []string{}, errors.Wrapf(err, "failed executing %s", decoderCmd)
	}

	// Example output format:
	// 0,          0,          0,        1,   115200, a5dad6170eb13fc5cbc6fe3511d44053
	// 0,          1,          1,        1,   115200, e056362baaf13dd0f888e67a681ab381
	// 0,          2,          2,        1,   115200, ee0c33d2b92e0443ca5770bd0c56911f
	var md5s []string
	for _, l := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if l[0] == '#' {
			continue
		}
		md5 := strings.TrimSpace(strings.Split(l, ", ")[5])
		md5s = append(md5s, md5)
	}
	return md5s, nil
}

func genMD5VPX(file, bin string) ([]string, error) {
	decoderCmd := bin + " -o output%w_%h_%4.yuv --i420 --md5"
	cmd := append(strings.Split(decoderCmd, " "), file)
	out, err := exec.Command(cmd[0], cmd[1:]...).Output()
	if err != nil {
		return []string{}, errors.Wrapf(err, "failed executing %s", decoderCmd)
	}

	// Example output format:
	// a6ddd21f5f4e7424b6e7a1f2925fb33b  output320_240_0001.yuv
	// 41c77adcfd29abfaad62a057855adeaa  output320_240_0002.yuv
	// afdb44531614034e4a4a90c805a5d3b1  output320_240_0003.yuv
	var md5s []string
	for _, l := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		md5 := strings.Split(l, " ")[0]
		md5s = append(md5s, md5)
	}
	return md5s, nil
}

func genMD5(file string, profile Profile) ([]string, error) {
	switch profileToCodec(profile) {
	case h264:
		return genMD5H264(file)
	case vp8, vp9:
		return genMD5VPX(file, "vpxdec")
	case av1:
		return genMD5VPX(file, "aomdec")
	}

	panic("Unknown codec: " + profileToString(profile))
	return []string{}, nil
}

func genJSONInfo(file string) (JSONInfo, error) {
	var info JSONInfo
	var profile Profile
	var err error
	if info.Width, info.Height, profile, info.BitDepth, err = parseStream(file); err != nil {
		return info, errors.Wrap(err, "failed parsing stream")
	}
	md5s, err := genMD5(file, profile)
	if err != nil {
		return info, errors.Wrap(err, "failed generating frame hashes")
	}

	info.Profile = profileToString(profile)
	info.NumFrames = len(md5s)
	info.MD5Checksums = md5s

	// Sets the framerate to 30 as the framerate value is not important in the
	// decoder test. Ideally, the framerate should be acquired by ffprobe result.
	info.FrameRate = 30

	// This is not probably correct for H264 streams. However, the value cannot
	// be known by ffprobe. The value needs to be found by running
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
	for _, file := range os.Args[1:] {
		ext := filepath.Ext(file)
		if ext != ".ivf" && ext != ".h264" {
			fmt.Println("The file's extension must be ivf or h264:", file)
			continue
		}

		info, err := genJSONInfo(file)
		if err != nil {
			fmt.Printf("failed getting json data for %s: %v\n", file, err)
			continue
		}

		jsonFile, err := writeJSON(file, info)
		if err != nil {
			fmt.Printf("failed writing json data for %s: %v\n", file, err)
			continue
		}

		fmt.Println("Created", jsonFile)
	}
}
