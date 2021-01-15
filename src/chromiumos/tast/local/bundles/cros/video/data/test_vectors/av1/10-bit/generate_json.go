// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// How to run
package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func IVFPaths(dir string) []string {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		panic(err)
	}

	var paths []string
	for _, file := range files {
		if file.IsDir() {
			paths = append(paths, IVFPaths(filepath.Join(dir, file.Name()))...)
		}
		ext := filepath.Ext(file.Name())
		if ext == ".ivf" {
			paths = append(paths, filepath.Join(dir, file.Name()))
		}
	}
	return paths
}

func genMD5(ivf string) []string {
	const decodeCmd = "aomdec -o output%w_%h_%4.yuv --i420 --md5"
	cmd := append(strings.Split(decodeCmd, " "), ivf)
	out, _ := exec.Command(cmd[0], cmd[1:]...).Output()
	// fmt.Println(string(out))
	var md5s []string
	for _, l := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		md5 := strings.Split(l, " ")[0]
		md5s = append(md5s, md5)
	}
	return md5s
}

func getInfo(ivf string) (int, int, bool) {
	const (
		ffprobCmd = "ffprobe -v quiet -print_format json -show_streams"
	)
	cmd := append(strings.Split(ffprobCmd, " "), ivf)
	out, _ := exec.Command(cmd[0], cmd[1:]...).Output()
	type InfoData struct {
		Streams []struct {
			CodecName string `json:"codec_name"`
			Profile   string `json:"profile"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
		} `json:"streams"`
	}

	var jsonData InfoData
	if err := json.Unmarshal(out, &jsonData); err != nil {
		fmt.Println(string(out))
		panic(err)
	}
	if len(jsonData.Streams) != 1 {
		fmt.Println("Please fix your self")
		return -1, -1, true
	}
	stream := jsonData.Streams[0]
	if stream.CodecName != "av1" {
		panic("No av1 video")
	}
	return stream.Width, stream.Height, stream.Profile == "Main"
}

func writeJson(ivf string, md5s []string) {
	type JsonInfo struct {
		Profile      string   `json:"profile"`
		BitDepth int `json:"bit_depth"`
		Width        int      `json:"width"`
		Height       int      `json:"height"`
		FrameRate    int      `json:"frame_rate"`
		NumFrames    int      `json:"num_frames"`
		NumFragments int      `json:"num_fragments"`
		MD5Checksums []string `json:"md5_checksums"`
	}
	var jsonData JsonInfo
	width, height, main := getInfo(ivf)
	if main {
		jsonData.Profile = "AV1PROFILE_PROFILE_MAIN"
	} else {
		jsonData.Profile = "AV1PROFILE_PROFILE_HIGH"
	}
	jsonData.Width = width
	jsonData.Height = height
	jsonData.FrameRate = 30
	jsonData.BitDepth = 10
	jsonData.NumFrames = len(md5s)
	jsonData.NumFragments = len(md5s)
	jsonData.MD5Checksums = md5s
	s, err := json.MarshalIndent(jsonData, "", "\t")
	if err != nil {
		panic(err)
	}
	// fmt.Println(string(s))
	jsonPath := ivf + ".json"
	ioutil.WriteFile(jsonPath, s, 0644)
}

func main() {
	if len(os.Args) != 2 {
		panic("invalid command line arguments")
	}
	rootDir, _ := filepath.Abs(os.Args[1])
	for _, ivf := range IVFPaths(rootDir) {
		fmt.Println(ivf)
		writeJson(ivf, genMD5(ivf))
	}
}
