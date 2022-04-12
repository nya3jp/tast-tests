// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

var wwcbIP = testing.RegisterVarString(
	"utils.WWCBIP",
	"192.168.1.198",
	"The ip of the wwcb server",
)

// HTTPGet sends HTTP request to server.
func HTTPGet(ctx context.Context, api string) (string, error) {
	// Send API request.
	url := fmt.Sprintf("http://%s:8585/%s", wwcbIP.Value(), api)
	testing.ContextLogf(ctx, "request: %s", url)
	resp, err := http.Get(url)
	if err != nil {
		return "", errors.Wrapf(err, "failed to send request %q", url)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", errors.Wrap(err, "failed to read response body")
	}
	// Check response.
	var data interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return "", errors.Wrap(err, "failed to parse data to json")
	}
	m := data.(map[string]interface{})
	if m["resultCode"] != "0000" {
		return "", errors.Errorf("unexpected response: %v", data)
	}
	testing.ContextLogf(ctx, "response: %s", data)
	return m["resultTxt"].(string), nil
}

// CloseAll disconnects all fixtures.
func CloseAll(ctx context.Context) error {
	api := fmt.Sprintf("api/closeall")
	if _, err := HTTPGet(ctx, api); err != nil {
		return errors.Wrap(err, "failed to disconnect all fixtures")
	}
	return nil
}

// SwitchFixture controls fixture to connect or disconnect.
// FixtureID: the ID of fixture.
// Action: control action, like "on", "off", "flip".
// Interval: delay time to execute on WWCB server.
func SwitchFixture(ctx context.Context, fixtureID, action, interval string) error {
	api := fmt.Sprintf("api/switchfixture?id=%s&action=%s&Interval=%s", fixtureID, action, interval)
	if _, err := HTTPGet(ctx, api); err != nil {
		return errors.Wrap(err, "failed to switch fixture")
	}
	return nil
}

// ControlAviosys controls aviosys power to turn on or off.
// PowerState: 1 means on / 0 means off.
// Port: there are 4 ports on aviosys power (e.g. 1,2,3,4).
func ControlAviosys(ctx context.Context, powerState, port string) error {
	api := fmt.Sprintf("api/AVIOSYS?type=%s&port=%s", powerState, port)
	if _, err := HTTPGet(ctx, api); err != nil {
		return errors.Wrap(err, "failed to control aviosys")
	}
	return nil
}

// GetPiColor returns color that WWCB server control camera to capture.
// FixtureID: the ID of fixture which controls display.
// Interval: delay time to execute on WWCB server.
func GetPiColor(ctx context.Context, fixtureID, interval string) (string, error) {
	api := fmt.Sprintf("api/getpicolor?id=%s&Interval=%s", fixtureID, interval)
	return HTTPGet(ctx, api)
}

// GetPiColorResult returns color that stored on WWCB server from the last execution of GetPiColor api.
func GetPiColorResult(ctx context.Context) (string, error) {
	api := fmt.Sprint("api/getpicolor_result")
	return HTTPGet(ctx, api)
}

// VideoRecord returns filepath that WWCB server control camera to record video.
// Durations: the lenght of time for recording video.
// FixtureID: the ID of fixture which controls display.
func VideoRecord(ctx context.Context, durations, fixtureID string) (string, error) {
	api := fmt.Sprintf("api/VideoRecord?durations=%s&id=%s&file_name=record&width=1280&height=720", durations, fixtureID)
	return HTTPGet(ctx, api)
}

// UploadFile uploads file to WWCB server through http post request.
// FilePath: the file stored on Chromebook.
func UploadFile(ctx context.Context, filePath string) (string, error) {
	url := fmt.Sprintf("http://%s:8585/api/upload_file", wwcbIP.Value())
	testing.ContextLogf(ctx, "request: %s", url)

	// Create body.
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("filename", filepath.Base(file.Name()))
	part, err := writer.CreateFormFile("file", filepath.Base(file.Name()))
	if err != nil {
		return "", errors.Wrap(err, "failed to create form file")
	}
	io.Copy(part, file)
	writer.Close()

	// Create POST request.
	request, err := http.NewRequest("POST", url, body)
	if err != nil {
		return "", errors.Wrap(err, "failed to new request")
	}
	request.Header.Add("Content-Type", writer.FormDataContentType())

	// Send request.
	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return "", errors.Wrap(err, "failed to send request")
	}
	defer response.Body.Close()
	content, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return "", errors.Wrap(err, "failed to parse response")
	}

	// Parse response.
	var data interface{}
	if err := json.Unmarshal(content, &data); err != nil {
		return "", errors.Wrap(err, "failed to parse data to json")
	}
	testing.ContextLogf(ctx, "response: %s", data)

	// Check response.
	m := data.(map[string]interface{})
	if m["resultCode"] != "0" && m["resultTxt"] != "success" && m["path"] != "" {
		return "", errors.New("failed to get correct response")
	}
	return m["path"].(string), nil
}

// DetectAudio detect certain words in audio file.
// FilePath: file path stored on WWCB server
func DetectAudio(ctx context.Context, filepath string) error {
	api := fmt.Sprintf("api/detect_audio?file_path=%s", filepath)
	if _, err := HTTPGet(ctx, api); err != nil {
		return err
	}
	return nil
}

// DetectVideo detect certain screens in video file.
// FilePath: file path stored on WWCB server.
func DetectVideo(ctx context.Context, filepath string) error {
	api := fmt.Sprintf("api/detect_video?file_path=%s", filepath)
	if _, err := HTTPGet(ctx, api); err != nil {
		return err
	}
	return nil
}
