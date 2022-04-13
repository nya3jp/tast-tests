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

// getWWCBURL retrieve wwcb web api URL from input var
func getWWCBURL(s *testing.State) string {

	// get input args
	URL, ok := s.Var(WWCBServerURL)
	if !ok {
		// default value
		webServerURL := "http://192.168.1.198"
		webServerPort := "8585"

		return fmt.Sprintf("%s:%s", webServerURL, webServerPort)
	}

	// in error, cannot contain colon
	http := "http://"
	if !strings.Contains(URL, http) {
		URL = http + URL
	}

	return URL
}

// AviosysControl control aviosys power
// Control the port power switch
// type:1 = on , 0 = off
// port:1.2.3.4
// etc.
// ip/api/AVIOSYS?type=1&port=1&port=2
func AviosysControl(s *testing.State, action, port string) error {

	// construct api api
	api := fmt.Sprintf("%s/api/AVIOSYS?type=%s&port=%s", getWWCBURL(s), action, port)

	s.Log("request: ", api)

	// send request
	res, err := http.Get(api)
	if err != nil {
		return errors.Wrapf(err, "failed to get response: %s", api)
	}

	// dispose when finished
	defer res.Body.Close()

	// get response
	body, err := ioutil.ReadAll(res.Body)
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
	s.Log("response: ", data)

	return nil
}

// SwitchFixture control switch fixture
// Control switch fixture
// Type:HDMI_Switch & TYPEC_Switch & TYPEA_Switch
// Index:ID1 & ID2 & ID3....
// cmd:
// HDMI,0:Close All;1:PortA;2:PortB;3:PortC;4:PortD
// Type-C,0:Close;1:Normal;2:Filp
// Type-A,1:PortA;2:PortB
// resultCode：0000 成功
// resultTxt：回應之訊息。
func SwitchFixture(s *testing.State, whatType, index, cmd, interval string) error {

	// construct URL
	URL := fmt.Sprintf("%s/api/switchfixture?Type=%s&Index=%s&cmd=%s&Interval=%s", getWWCBURL(s), whatType, index, cmd, interval)

	s.Log("request: ", URL)

	// send request
	res, err := http.Get(URL)
	if err != nil {
		return errors.Wrapf(err, "failed to get response: %s", URL)
	}
	// dispose when finished
	defer res.Body.Close()

	// get response
	body, err := ioutil.ReadAll(res.Body)
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
	s.Log("response: ", data)

	return nil
}

// GetPiColor get specific area on camera return colr
// Use the camera to detect the preset range of colors
// Interval(可選):延遲n秒後取得顏色，使用延遲會立刻回傳resultCode:0000、resultTxt:None
// resultCode：0000 成功、0001 參數格式有誤、0002 執行失敗
// resultTxt：回應之訊息。
// 偵測成功會回傳偵測到的顏色etc. red
func GetPiColor(s *testing.State, dispType, dispIndex, interval string) (string, error) {

	URL := fmt.Sprintf("%s/api/getpicolor?DisplayType=%s&ID=%s&Interval=%s", getWWCBURL(s), dispType, dispIndex, interval)

	s.Log("request: ", URL)

	// send request
	res, err := http.Get(URL)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get response: %s", URL)
	}

	// dispose when finished
	defer res.Body.Close()

	// get response
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", errors.Wrap(err, "failed to read all response")
	}

	// parse response
	var data interface{} // TopTracks
	if err := json.Unmarshal(body, &data); err != nil {
		return "", errors.Wrap(err, "failed to parse data to json ")
	}

	s.Log("response: ", data)

	// check response
	m := data.(map[string]interface{})
	if m["resultCode"] != "0000" {
		return "", errors.Errorf("response is not correctly, got %s, want 0000", m["resultCode"])
	}

	var color string
	color = fmt.Sprintf("%v", m["resultTxt"])

	return color, nil
}

// GetPiColorResult get color from specific area on camera
// Return camera detect the preset range of colors by background task
// resultTxt:None,background task an error or Color not yet obtained
// resultCode：0000 成功、0001 參數格式有誤、0002 執行失敗
// resultTxt：回應之訊息。
// 偵測成功會回傳偵測到的顏色etc. red，回傳None代表執行失敗或者尚未取得顏色
func GetPiColorResult(s *testing.State) (string, error) {

	URL := fmt.Sprintf("%s/api/getpicolor_result", getWWCBURL(s))

	s.Log("request: ", URL)

	// send request
	res, err := http.Get(URL)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get response: %s", URL)
	}

	// dispose when finished
	defer res.Body.Close()

	// get response
	body, err := ioutil.ReadAll(res.Body)
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
		return "", errors.Errorf("response is not correctly, got %s, want 0000", m["resultCode"])
	}

	s.Log("response: ", data)

	var color string
	color = fmt.Sprintf("%s", m["resultTxt"])

	return color, nil
}

// VideoRecord let server to do video record
// Use the default camera and mic record video
// Output:Video save path,do not include file name
// etc. 20210831-142730
// Durations:record time
// DisplayType(option):Display fixture type, default is Display_HDMI_Switch
// ID(option):Display fixture id, default is ID1
// Width(可選):預設為1920
// Height(可選):預設為1080
// resultCode：0000 成功、0001 參數格式有誤、0002 執行失敗
// resultTxt：錄製成功回傳影片完整路徑
func VideoRecord(s *testing.State, durations, output, dispType, dispIndex string) (string, error) {

	URL := fmt.Sprintf("%s/api/VideoRecord?Durations=%s&Output=%s&DisplayType=%s&ID=%s&Width=1280&Height=720", getWWCBURL(s), durations, output, dispType, dispIndex)

	s.Log("request: ", URL)

	// send request
	res, err := http.Get(URL)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get response: %s", URL)
	}

	// dispose when finished
	defer res.Body.Close()

	// get response
	body, err := ioutil.ReadAll(res.Body)
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
		return "", errors.Errorf("response is not correctly, got %s, want 0000", m["resultCode"])
	}

	var filepath string
	filepath = m["resultTxt"].(string)

	// print response
	s.Log("response: ", data)

	return filepath, nil
}

// GoldenPredict golden predict
func GoldenPredict(s *testing.State, videoPath, dispType, dispIndex string, audio bool) error {

	/*
		Predict golden sample and input video

		The video resolution must be greater than 1280*720

		Input:full video path include file name

		Audio:true;predict audio only,false(default);predict video and audio

		etc. /home/allion/Videos/testvideo/normal.mp4

		DisplayType(option):Display fixture type, default is Display_HDMI_Switch

		ID(option):Display fixture id, default is ID1

		resultCode：0000 成功、0001 參數格式有誤、0002 執行失敗

		resultTxt：video、audio都預測成功回傳pass，其中一個失敗會回傳video fail or audio fail，兩個都失敗則回傳video and audio fail

		http://server:port/api/goldenpredict?Input=/home/allion/gui-env/testvideo/issue.mp4
	*/

	// construct URL
	URL := fmt.Sprintf("%s/api/goldenpredict?Input=%s&DisplayType=%s&ID=%s&Audio=%t", getWWCBURL(s), videoPath, dispType, dispIndex, audio)

	s.Log("request: ", URL)

	// send request
	res, err := http.Get(URL)
	if err != nil {
		return errors.Wrapf(err, "failed to get response: %s", URL)
	}
	// dispose when finished
	defer res.Body.Close()

	// get response
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return errors.Wrap(err, "failed to read all response")
	}

	// parse response
	var data interface{} // TopTracks
	if err := json.Unmarshal(body, &data); err != nil {
		return errors.Wrap(err, "failed to parse data to json")
	}

	s.Log("response: ", data)

	// check response
	m := data.(map[string]interface{})

	if m["resultCode"] != "0000" || m["resultTxt"] != "Pass" {
		return errors.New("failed to get correct response: ")
	}

	return nil
}

// WebNotification pop up msg on wwcb server
func WebNotification(s *testing.State, msg string) error {

	// replace space with _
	if strings.Contains(msg, " ") {
		msg = strings.ReplaceAll(msg, " ", "_")
	}

	// construct URL
	URL := fmt.Sprintf("%s/api/notice_msg?msg=%s", getWWCBURL(s), msg)

	s.Log("request: ", URL)

	// send request
	res, err := http.Get(URL)
	if err != nil {
		return errors.Wrap(err, "failed to get response")
	}
	// dispose when finished
	defer res.Body.Close()

	// get response
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return errors.Wrap(err, "failed to read all response")
	}

	// parse response
	var data interface{} // TopTracks
	if err := json.Unmarshal(body, &data); err != nil {
		return errors.Wrap(err, "failed to parse data to json")
	}

	s.Log("response: ", data)

	// check response
	m := data.(map[string]interface{})

	if m["resultCode"] != "0" || m["resultTxt"] != "success" {
		return errors.New("failed to get correct response: ")
	}

	return nil
}

// GetFile get file from WWCB server
func GetFile(ctx context.Context, s *testing.State, storepath, filename string) error {

	s.Log("Get server file ")

	serverFile := fmt.Sprintf("%s/%s/%s", getWWCBURL(s), "script_upload", filename)

	getFile := testexec.CommandContext(ctx, "wget", "-P", storepath, serverFile)

	_, err := getFile.Output(testexec.DumpLogOnError)
	if err != nil {
		return errors.Wrapf(err, "%q failed: ", shutil.EscapeSlice(getFile.Args))
	}

	return nil
}

// UploadFile upload file to wwcb server through http request - POST
func UploadFile(s *testing.State, filename string) (string, error) {

	URL := fmt.Sprintf("%s/api/upload_file", getWWCBURL(s))

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

	request, err := http.NewRequest("POST", URL, body)
	if err != nil {
		return "", errors.Wrap(err, "failed to new request")
	}

	request.Header.Add("Content-Type", writer.FormDataContentType())

	s.Log("request: ", URL)

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

	s.Log("response: ", data)

	// check response
	m := data.(map[string]interface{})
	if m["resultCode"] != "0" && m["resultTxt"] != "success" && m["path"] != "" {
		return "", errors.New("failed to get correct response: ")
	}

	return m["path"].(string), nil
}
