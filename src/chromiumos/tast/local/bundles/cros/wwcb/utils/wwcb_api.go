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

// HTTPGetRequest send HTTP request to server
func HTTPGetRequest(ctx context.Context, api string) (string, error) {
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

// DisconnectAllFixtures tell server to disconnect all fixtures
func DisconnectAllFixtures(ctx context.Context) error {
	api := fmt.Sprintf("api/closeall")
	_, err := HTTPGetRequest(ctx, api)
	if err != nil {
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
	_, err := HTTPGetRequest(ctx, api)
	if err != nil {
		return errors.Wrap(err, "failed to switch fixture")
	}
	return nil
}

// ControlAviosys control aviosys power, send api like this :ip/api/AVIOSYS?type=1&port=1&port=2
// explain parameters
// powerState: (avoid keyword like "type" in api) 1 means turning on / 2 means turning off
// port: there is 4 port on Aviosys power, select port to open or close, like port 1,2,3,4
func ControlAviosys(ctx context.Context, powerState, port string) error {
	api := fmt.Sprintf("api/AVIOSYS?type=%s&port=%s", powerState, port)
	_, err := HTTPGetRequest(ctx, api)
	if err != nil {
		return errors.Wrap(err, "failed to control aviosys")
	}
	return nil
}

// GetPiColor return color from wwcb server by using camera to capture a picture
// explain parameters
// dispType means what kind of display, like Display_HDMI_Switch or Display_DP_Switch
// dispIndex means what index of this kind "dispType" display, like ID1, ID2
// interval means how much time to delay that wwcb server execute function after it accept api
func GetPiColor(ctx context.Context, cameraID, interval string) (string, error) {
	api := fmt.Sprintf("api/getpicolor?id=%s&Interval=%s", cameraID, interval)
	color, err := HTTPGetRequest(ctx, api)
	if err != nil {
		return "", err
	}
	return color, nil
}

// GetPiColorResult return color that stored from last executed getPiColor api in wwcb server
func GetPiColorResult(ctx context.Context) (string, error) {
	api := fmt.Sprint("api/getpicolor_result")
	color, err := HTTPGetRequest(ctx, api)
	if err != nil {
		return "", err
	}
	return color, nil
}

// VideoRecord return filepath that server let camera record video that store in tast result folder
// durations means how long camera record video time length
// output means video storage path
// dispIndex means what index of this kind "dispType" display, like ID1, ID2 ..
func VideoRecord(ctx context.Context, durations, filepath, id string) (string, error) {
	api := fmt.Sprintf("api/VideoRecord?Durations=%s&Output=%s&id=%s&Width=1280&Height=720", durations, filepath, id)
	filepath, err := HTTPGetRequest(ctx, api)
	if err != nil {
		return "", err
	}
	return filepath, nil
}

// GoldenPredict compare video with godlen sample
// videoPath means relative path in tast result folder, like /result/20220524-151453
// id means the id of camera
// audio is boolean, true means predict audio only, false means video and audio
func GoldenPredict(ctx context.Context, videoPath, id string, audio bool) error {
	api := fmt.Sprintf("api/goldenpredict?Input=%s&id=%s&Audio=%t", videoPath, id, audio)
	_, err := HTTPGetRequest(ctx, api)
	if err != nil {
		return err
	}
	return nil
}

// GetFile get file from WWCB server
// fileName means the want file's filename on wwcb server under "script_upload" folder
// storePath means file path on Chromebook
func GetFile(ctx context.Context, storepath, filename string) error {
	testing.ContextLog(ctx, "Get server file")

	serverFile := fmt.Sprintf("http://%s:8585/%s/%s", wwcbIP.Value(), "script_upload", filename)

	getFile := testexec.CommandContext(ctx, "wget", "-P", storepath, serverFile)
	_, err := getFile.Output(testexec.DumpLogOnError)
	if err != nil {
		return errors.Wrapf(err, "%q failed", shutil.EscapeSlice(getFile.Args))
	}
	return nil
}

// UploadFile upload file to wwcb server through http request - POST
// filename means the file store on Chromebook
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
