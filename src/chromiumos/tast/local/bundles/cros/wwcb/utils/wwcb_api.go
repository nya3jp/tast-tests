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
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

// wwcbURL retrieves wwcb web url from input var
func wwcbURL(s *testing.State) (string, error) {

	// get input args
	url, ok := s.Var(WWCBServerURL)
	if !ok {
		return "", errors.New("wwcb url is missing")
	}

	// in case, user forget to add http://
	http := "http://"
	if !strings.Contains(url, http) {
		url = http + url
	}

	return url, nil
}

// AviosysControl control aviosys power, send api like this :ip/api/AVIOSYS?type=1&port=1&port=2
// explain parameters
// powerState: (avoid keyword like "type" in api) 1 stand for turn on / 2 stand for turn off
// port: there is 4 port on Aviosys power, select port to open or close, like port 1,2,3,4
func AviosysControl(ctx context.Context, s *testing.State, powerState, port string) error {

	ip, err := wwcbURL(s)
	if err != nil {
		return err
	}

	// construct url
	url := fmt.Sprintf("%s/api/AVIOSYS?type=%s&port=%s", ip, powerState, port)

	testing.ContextLogf(ctx, "request: %s", url)

	// send request
	response, err := http.Get(url)
	if err != nil {
		return errors.Wrapf(err, "failed to get response: %s", url)
	}

	// dispose when finished
	defer response.Body.Close()

	// get response
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return errors.Wrap(err, "failed to read all response")
	}

	// parse response
	var data interface{} // TopTracks
	if err := json.Unmarshal(body, &data); err != nil {
		return errors.Wrap(err, "failed to parse data to json")
	}

	// check response
	m := data.(map[string]interface{})
	if m["resultCode"] != "0000" || m["resultTxt"] != "Success" {
		return errors.Errorf("failed to check response: %v", data)
	}

	// print response
	testing.ContextLogf(ctx, "response: %s", data)

	return nil
}

// SwitchFixture control switch fixture
// explain parameters
// fixtureType means what kind of fixture to switch, like HDMI_Switch, TYPEA_Switch (avoid keyword "type" in api)
// fixtureIndex means what index of this "fixtureType" fixture, there is setup config in wwcb server, like ID1, ID2 ...
// switchCmd has different meaning on fixtures, some switchCmd is to control fixtures to do action, like open or close fixture, the others is just to open which port on fixture
// interval meands when wwcb server get the api call, what time to delay then execute switch fixture method
func SwitchFixture(ctx context.Context, s *testing.State, fixtureType, fixtureIndex, switchCmd, interval string) error {

	ip, err := wwcbURL(s)
	if err != nil {
		return err
	}

	// construct url
	url := fmt.Sprintf("%s/api/switchfixture?Type=%s&Index=%s&cmd=%s&Interval=%s", ip, fixtureType, fixtureIndex, switchCmd, interval)

	testing.ContextLogf(ctx, "request: %s", url)

	// send request
	response, err := http.Get(url)
	if err != nil {
		return errors.Wrapf(err, "failed to get response: %s", url)
	}
	// dispose when finished
	defer response.Body.Close()

	// get response
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return errors.Wrap(err, "failed to read all response")
	}

	// parse response
	var data interface{} // TopTracks
	if err := json.Unmarshal(body, &data); err != nil {
		return errors.Wrap(err, "failed to parse data to json")
	}

	// check response
	m := data.(map[string]interface{})
	// notice : is "Success "
	if m["resultCode"] != "0000" || m["resultTxt"] != "Success" {
		return errors.Errorf("failed to check response: %v", data)
	}

	// print response
	testing.ContextLogf(ctx, "response: %s", data)

	return nil
}

// GetPiColor return color from wwcb server by using camera to capture a picture
// explain parameters
// dispType means what kind of display, like Display_HDMI_Switch or Display_DP_Switch
// dispIndex means what index of this kind "dispType" display, like ID1, ID2
// interval means how much time to delay that wwcb server execute function after it accept api
func GetPiColor(ctx context.Context, s *testing.State, dispType, dispIndex, interval string) (string, error) {

	ip, err := wwcbURL(s)
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("%s/api/getpicolor?DisplayType=%s&ID=%s&Interval=%s", ip, dispType, dispIndex, interval)

	testing.ContextLogf(ctx, "request: %s", url)

	// send request
	response, err := http.Get(url)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get response: %s", url)
	}

	// dispose when finished
	defer response.Body.Close()

	// get response
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return "", errors.Wrap(err, "failed to read all response")
	}

	// parse response
	var data interface{} // TopTracks
	if err := json.Unmarshal(body, &data); err != nil {
		return "", errors.Wrap(err, "failed to parse data to json ")
	}

	testing.ContextLogf(ctx, "response: %s", data)

	// check response
	m := data.(map[string]interface{})
	if m["resultCode"] != "0000" {
		return "", errors.Errorf("response is not correct; got %s, want 0000", m["resultCode"])
	}

	color := fmt.Sprintf("%v", m["resultTxt"])

	return color, nil
}

// GetPiColorResult return color that stored from last executed getPiColor api in wwcb server
func GetPiColorResult(ctx context.Context, s *testing.State) (string, error) {

	ip, err := wwcbURL(s)
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("%s/api/getpicolor_result", ip)

	testing.ContextLogf(ctx, "request: %s", url)

	// send request
	response, err := http.Get(url)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get response: %s", url)
	}

	// dispose when finished
	defer response.Body.Close()

	// get response
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return "", errors.Wrap(err, "failed to read all response")
	}

	// parse response
	var data interface{} // TopTracks
	if err := json.Unmarshal(body, &data); err != nil {
		return "", errors.Wrap(err, "failed to parse data to json")
	}

	// check response
	m := data.(map[string]interface{})
	if m["resultCode"] != "0000" {
		return "", errors.Errorf("response is not correct; got %s, want 0000", m["resultCode"])
	}

	testing.ContextLogf(ctx, "response: %s", data)

	color := fmt.Sprintf("%s", m["resultTxt"])

	return color, nil
}

// VideoRecord tell wwcb server to let camera record video, then store in tast result folder
// durations means how long camera record video time length
// output means video storage path
// dispType means what kind of display, like Display_HDMI_Switch or Display_DP_Switch
// dispIndex means what index of this kind "dispType" display, like ID1, ID2 ..
func VideoRecord(ctx context.Context, s *testing.State, durations, output, dispType, dispIndex string) (string, error) {

	ip, err := wwcbURL(s)
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("%s/api/VideoRecord?Durations=%s&Output=%s&DisplayType=%s&ID=%s&Width=1280&Height=720", ip, durations, output, dispType, dispIndex)

	testing.ContextLogf(ctx, "request: %s", url)

	// send request
	response, err := http.Get(url)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get response: %s", url)
	}

	// dispose when finished
	defer response.Body.Close()

	// get response
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return "", errors.Wrap(err, "failed to read all response")
	}

	// parse response
	var data interface{} // TopTracks
	if err := json.Unmarshal(body, &data); err != nil {
		return "", errors.Wrap(err, "failed to parse data to json")
	}

	// check response
	m := data.(map[string]interface{})
	if m["resultCode"] != "0000" {
		return "", errors.Errorf("response is not correct; got %s, want 0000", m["resultCode"])
	}

	var filepath string
	filepath = m["resultTxt"].(string)

	// print response
	testing.ContextLogf(ctx, "response: %s", data)

	return filepath, nil
}

// GoldenPredict compare video with godlen sample
// videoPath means relative path in tast result folder, like /result/20220524-151453
// dispType means what kind of display, like Display_HDMI_Switch or Display_DP_Switch
// dispIndex means what index of this kind display, like ID1, ID2 ..
// audio is boolean, true means predict audio only, false means video and audio
func GoldenPredict(ctx context.Context, s *testing.State, videoPath, dispType, dispIndex string, audio bool) error {

	ip, err := wwcbURL(s)
	if err != nil {
		return err
	}

	// construct url
	url := fmt.Sprintf("%s/api/goldenpredict?Input=%s&DisplayType=%s&ID=%s&Audio=%t", ip, videoPath, dispType, dispIndex, audio)

	testing.ContextLogf(ctx, "request: %s", url)

	// send request
	response, err := http.Get(url)
	if err != nil {
		return errors.Wrapf(err, "failed to get response: %s", url)
	}
	// dispose when finished
	defer response.Body.Close()

	// get response
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return errors.Wrap(err, "failed to read all response")
	}

	// parse response
	var data interface{} // TopTracks
	if err := json.Unmarshal(body, &data); err != nil {
		return errors.Wrap(err, "failed to parse data to json")
	}

	testing.ContextLogf(ctx, "response: %s", data)

	// check response
	m := data.(map[string]interface{})

	if m["resultCode"] != "0000" || m["resultTxt"] != "Pass" {
		return errors.New("failed to get correct response: ")
	}

	return nil
}

// WebNotification pop up msg on wwcb server
// msg means what message you wanna show on wwcb server
func WebNotification(ctx context.Context, s *testing.State, msg string) error {

	ip, err := wwcbURL(s)
	if err != nil {
		return err
	}

	// replace space with _
	if strings.Contains(msg, " ") {
		msg = strings.ReplaceAll(msg, " ", "_")
	}

	// construct url
	url := fmt.Sprintf("%s/api/notice_msg?msg=%s", ip, msg)

	testing.ContextLogf(ctx, "request: %s", url)

	// send request
	response, err := http.Get(url)
	if err != nil {
		return errors.Wrap(err, "failed to get response")
	}
	// dispose when finished
	defer response.Body.Close()

	// get response
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return errors.Wrap(err, "failed to read all response")
	}

	// parse response
	var data interface{} // TopTracks
	if err := json.Unmarshal(body, &data); err != nil {
		return errors.Wrap(err, "failed to parse data to json")
	}

	testing.ContextLogf(ctx, "response: %s", data)

	// check response
	m := data.(map[string]interface{})

	if m["resultCode"] != "0" || m["resultTxt"] != "success" {
		return errors.New("failed to get correct response: ")
	}

	return nil
}

// GetFile get file from WWCB server
// fileName means the want file's filename on wwcb server under "script_upload" folder
// storePath means file path on Chromebook
func GetFile(ctx context.Context, s *testing.State, storepath, filename string) error {

	testing.ContextLog(ctx, "Get server file")

	ip, err := wwcbURL(s)
	if err != nil {
		return err
	}

	serverFile := fmt.Sprintf("%s/%s/%s", ip, "script_upload", filename)

	getFile := testexec.CommandContext(ctx, "wget", "-P", storepath, serverFile)

	_, err = getFile.Output(testexec.DumpLogOnError)
	if err != nil {
		return errors.Wrapf(err, "%q failed", shutil.EscapeSlice(getFile.Args))
	}

	return nil
}

// UploadFile upload file to wwcb server through http request - POST
// filename means the file store on Chromebook
func UploadFile(ctx context.Context, s *testing.State, filename string) (string, error) {

	ip, err := wwcbURL(s)
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("%s/api/upload_file", ip)

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

// ResetAllFixtures disconnect all fixtures in the end of testing
func ResetAllFixtures(ctx context.Context, s *testing.State) error {
	ip, err := wwcbURL(s)
	if err != nil {
		return err
	}

	// construct url
	url := fmt.Sprintf("%s/api/closeAll", ip)

	testing.ContextLogf(ctx, "request: %s", url)

	// send request
	response, err := http.Get(url)
	if err != nil {
		return errors.Wrapf(err, "failed to get response: %s", url)
	}
	// dispose when finished
	defer response.Body.Close()

	// get response
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return errors.Wrap(err, "failed to read all response")
	}

	// parse response
	var data interface{} // TopTracks
	if err := json.Unmarshal(body, &data); err != nil {
		return errors.Wrap(err, "failed to parse data to json")
	}

	// check response
	m := data.(map[string]interface{})
	// notice : is "Success "
	if m["resultCode"] != "0000" || m["resultTxt"] != "Success" {
		return errors.Errorf("failed to check response: %v", data)
	}

	// print response
	testing.ContextLogf(ctx, "response: %s", data)

	return nil
}
