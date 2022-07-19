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

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

var wwcbIP = testing.RegisterVarString(
	"utils.WWCBIP",
	"192.168.1.198",
	"The ip of the wwcb server",
)

// HTTPGet send HTTP request to server
func HTTPGet(ctx context.Context, api string) (string, error) {
	url := fmt.Sprintf("http://%s:8585/%s", wwcbIP.Value(), api)
	testing.ContextLogf(ctx, "request: %s", url)
	resp, err := http.Get(url)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get %q", url)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", errors.Wrap(err, "failed to read response body")
	}
	// check response
	var data interface{} // TopTracks
	if err := json.Unmarshal(body, &data); err != nil {
		return "", errors.Wrap(err, "failed to parse data to json")
	}
	m := data.(map[string]interface{})
	if m["resultCode"] != "0000" {
		return "", errors.Errorf("failed to get correct response: %v", data)
	}
	testing.ContextLogf(ctx, "response: %s", data)
	return m["resultTxt"].(string), nil
}

// CloseAll tell server to disconnect all fixtures
func CloseAll(ctx context.Context) error {
	api := fmt.Sprintf("api/closeall")
	if _, err := HTTPGet(ctx, api); err != nil {
		return errors.Wrap(err, "failed to disconnect all fixtures")
	}
	return nil
}

// SwitchFixture control switch fixture
// explain parameters
// id repesent fixture's id on the physical fixture
// action repesent that make fixture what to do, like "on", "off", "flip"
// interval means when wwcb server get the api call, how much time to delay then execute switch fixture method
func SwitchFixture(ctx context.Context, id, action, interval string) error {
	api := fmt.Sprintf("api/switchfixture?id=%s&action=%s&Interval=%s", id, action, interval)
	if _, err := HTTPGet(ctx, api); err != nil {
		return errors.Wrap(err, "failed to switch fixture")
	}
	return nil
}

// ControlAviosys control aviosys power, send api like this :ip/api/AVIOSYS?type=1&port=1&port=2
// explain parameters
// powerState: 1 means turning on / 0 means turning off (avoid keyword like "type" in api)
// port: there is 4 port on Aviosys power, select port to open or close, like port 1,2,3,4
func ControlAviosys(ctx context.Context, powerState, port string) error {
	api := fmt.Sprintf("api/AVIOSYS?type=%s&port=%s", powerState, port)
	if _, err := HTTPGet(ctx, api); err != nil {
		return errors.Wrap(err, "failed to control aviosys")
	}
	return nil
}

// GetPiColor return color from wwcb server by using camera to capture a picture
// explain parameters
// cameraID means the ID of camera
// interval means how much time to delay that wwcb server execute function after it accept api
func GetPiColor(ctx context.Context, cameraID, interval string) (string, error) {
	api := fmt.Sprintf("api/getpicolor?id=%s&Interval=%s", cameraID, interval)
	return HTTPGet(ctx, api)
}

// GetPiColorResult return color that stored from last executed getPiColor api in wwcb server
func GetPiColorResult(ctx context.Context) (string, error) {
	api := fmt.Sprint("api/getpicolor_result")
	return HTTPGet(ctx, api)
}

// VideoRecord return filepath that server let camera record video that store in tast result folder
// durations means how long camera record video time length
// cameraID means the ID of camera
func VideoRecord(ctx context.Context, durations, cameraID string) (string, error) {
	api := fmt.Sprintf("api/VideoRecord?durations=%s&id=%s&file_name=record&width=1280&height=720", durations, cameraID)
	return HTTPGet(ctx, api)
}

// GetFile get file from WWCB server
// storePath means file path on Chromebook
// fileName means the want file's filename on wwcb server under "script_upload" folder
func GetFile(ctx context.Context, storepath, filename string) error {
	testing.ContextLog(ctx, "Get server file")

	serverFile := fmt.Sprintf("http://%s:8585/%s/%s", wwcbIP.Value(), "script_upload", filename)
	getFile := testexec.CommandContext(ctx, "wget", "-P", storepath, serverFile)
	if _, err := getFile.Output(testexec.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "%q failed", shutil.EscapeSlice(getFile.Args))
	}
	return nil
}

// UploadFile upload file to wwcb server through http request - POST
// filename means the file stored on Chromebook
func UploadFile(ctx context.Context, filename string) (string, error) {
	url := fmt.Sprintf("http://%s:8585/api/upload_file", wwcbIP.Value())

	file, err := os.Open(filename)
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

	request, err := http.NewRequest("POST", url, body)
	if err != nil {
		return "", errors.Wrap(err, "failed to new request")
	}
	request.Header.Add("Content-Type", writer.FormDataContentType())

	testing.ContextLogf(ctx, "request: %s", url)

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

	// parse response
	var data interface{} // TopTracks
	if err := json.Unmarshal(content, &data); err != nil {
		return "", errors.Wrap(err, "failed to parse data to json")
	}

	testing.ContextLogf(ctx, "response: %s", data)

	// check response
	m := data.(map[string]interface{})
	if m["resultCode"] != "0" && m["resultTxt"] != "success" && m["path"] != "" {
		return "", errors.New("failed to get correct response")
	}
	return m["path"].(string), nil
}

// DetectAudio do audio comparison with recording audio file that upload to server
func DetectAudio(ctx context.Context, filepath string) error {
	api := fmt.Sprintf("api/detect_audio?file_path=%s", filepath)
	if _, err := HTTPGet(ctx, api); err != nil {
		return err
	}
	return nil
}

// DetectVideo do video comparison with recording video file by wwcb server
func DetectVideo(ctx context.Context, filepath string) error {
	api := fmt.Sprintf("api/detect_video?file_path=%s", filepath)
	if _, err := HTTPGet(ctx, api); err != nil {
		return err
	}
	return nil
}
