/*

Copyright (C) 2017-2018  Ettore Di Giacinto <mudler@gentoo.org>
Some code portions and re-implemented design are also coming
from the Gogs project, which is using the go-macaron framework and was
really source of ispiration. Kudos to them!

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program. If not, see <http://www.gnu.org/licenses/>.

*/

package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"strings"

	setting "github.com/MottainaiCI/mottainai-server/pkg/settings"

	"github.com/mudler/anagent"
)

type Fetcher struct {
	BaseURL       string
	docID         string
	Agent         *anagent.Anagent
	ActiveReports bool
}

func NewClient(host string) *Fetcher {
	return &Fetcher{BaseURL: host}
}

func NewFetcher(docID string) *Fetcher {
	f := NewClient(setting.Configuration.AppURL)
	f.docID = docID
	return f
}

func New(docID string, a *anagent.Anagent) *Fetcher {
	f := NewClient(setting.Configuration.AppURL)
	f.docID = docID
	f.Agent = a
	return f
}

func (f *Fetcher) Doc(id string) {
	f.docID = id
}

func (f *Fetcher) GetJSONOptions(url string, option map[string]string, target interface{}) error {
	hclient := &http.Client{}
	request, err := http.NewRequest("GET", f.BaseURL+url, nil)
	if err != nil {
		return err
	}

	q := request.URL.Query()
	for k, v := range option {
		q.Add(k, v)
	}
	request.URL.RawQuery = q.Encode()
	if err != nil {
		return err
	}

	response, err := hclient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	return json.NewDecoder(response.Body).Decode(target)
}

func (f *Fetcher) GetOptions(url string, option map[string]string) ([]byte, error) {
	hclient := &http.Client{}
	request, err := http.NewRequest("GET", f.BaseURL+url, nil)
	if err != nil {
		return []byte{}, err
	}

	q := request.URL.Query()
	for k, v := range option {
		q.Add(k, v)
	}
	request.URL.RawQuery = q.Encode()
	if err != nil {
		return []byte{}, err
	}

	response, err := hclient.Do(request)
	if err != nil {
		return []byte{}, err
	}
	defer response.Body.Close()

	contents, err := ioutil.ReadAll(response.Body)
	return contents, err
}

func (f *Fetcher) GenericForm(URL string, option map[string]interface{}) ([]byte, error) {
	hclient := &http.Client{}
	form := url.Values{}
	var InterfaceList []interface{}

	for k, v := range option {
		if reflect.TypeOf(v) == reflect.TypeOf(InterfaceList) {
			for _, el := range v.([]interface{}) {
				form.Add(k, el.(string))
			}
		} else {
			form.Add(k, v.(string))
		}
	}

	request, err := http.NewRequest("POST", f.BaseURL+URL, strings.NewReader(form.Encode()))
	if err != nil {
		return []byte{}, err
	}

	request.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	response, err := hclient.Do(request)
	if err != nil {
		return []byte{}, err
	}
	defer response.Body.Close()

	contents, err := ioutil.ReadAll(response.Body)
	return contents, err
}

func (f *Fetcher) Form(URL string, option map[string]string) ([]byte, error) {
	hclient := &http.Client{}

	form := url.Values{}
	for k, v := range option {
		form.Add(k, v)
	}

	request, err := http.NewRequest("POST", f.BaseURL+URL, strings.NewReader(form.Encode()))
	if err != nil {
		return []byte{}, err
	}
	//request.Header.Add("Content-Type", writer.FormDataContentType())

	request.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	// q := request.URL.Query()
	// for k, v := range option {
	// 	q.Add(k, v)
	// }
	// request.URL.RawQuery = q.Encode()
	// if err != nil {
	// 	return []byte{}, err
	// }

	response, err := hclient.Do(request)
	if err != nil {
		return []byte{}, err
	}
	defer response.Body.Close()

	contents, err := ioutil.ReadAll(response.Body)
	return contents, err
}

func (f *Fetcher) PostOptions(URL string, option map[string]string) ([]byte, error) {
	hclient := &http.Client{}

	form := url.Values{}
	for k, v := range option {
		form.Add(k, v)
	}

	request, err := http.NewRequest("POST", f.BaseURL+URL, strings.NewReader(form.Encode()))
	if err != nil {
		return []byte{}, err
	}
	//request.Header.Add("Content-Type", writer.FormDataContentType())

	request.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	q := request.URL.Query()
	for k, v := range option {
		q.Add(k, v)
	}
	request.URL.RawQuery = q.Encode()
	if err != nil {
		return []byte{}, err
	}

	response, err := hclient.Do(request)
	if err != nil {
		return []byte{}, err
	}
	defer response.Body.Close()

	contents, err := ioutil.ReadAll(response.Body)
	return contents, err
}

func (f *Fetcher) UploadLargeFile(uri string, params map[string]string, paramName string, filePath string, chunkSize int) error {
	//open file and retrieve info
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	fi, err := file.Stat()
	if err != nil {
		return err
	}
	defer file.Close()

	//buffer for storing multipart data
	byteBuf := &bytes.Buffer{}

	//part: parameters
	mpWriter := multipart.NewWriter(byteBuf)
	for key, value := range params {
		err = mpWriter.WriteField(key, value)
		if err != nil {
			return err
		}
	}

	//part: file
	mpWriter.CreateFormFile(paramName, fi.Name())
	contentType := mpWriter.FormDataContentType()

	nmulti := byteBuf.Len()
	multi := make([]byte, nmulti)
	_, err = byteBuf.Read(multi)
	if err != nil {
		return err
	}
	//part: latest boundary
	//when multipart closed, latest boundary is added
	mpWriter.Close()
	nboundary := byteBuf.Len()
	lastBoundary := make([]byte, nboundary)
	_, err = byteBuf.Read(lastBoundary)
	if err != nil {
		return err
	}

	//use pipe to pass request
	rd, wr := io.Pipe()
	defer rd.Close()

	go func() {
		defer wr.Close()

		//write multipart
		_, _ = wr.Write(multi)

		//write file
		buf := make([]byte, chunkSize)
		for {
			n, err := file.Read(buf)
			if err != nil {
				break
			}
			_, _ = wr.Write(buf[:n])
		}
		//write boundary
		_, _ = wr.Write(lastBoundary)
	}()

	//construct request with rd
	req, err := http.NewRequest("POST", f.BaseURL+uri, rd)
	if err != nil {
		return err
	}
	req.TransferEncoding = []string{"chunked"}

	req.Header.Set("Content-Type", contentType)
	req.ContentLength = -1 //totalSize
	req.ProtoMajor = 1
	req.ProtoMinor = 1
	req.Header.Add("Connection", "keep-alive")

	//process request
	client := &http.Client{Timeout: 0}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	} else {
		log.Println(resp.StatusCode)
		log.Println(resp.Header)

		body := &bytes.Buffer{}
		_, _ = body.ReadFrom(resp.Body)
		resp.Body.Close()
		log.Println(body)
		if resp.StatusCode != 200 {
			return errors.New("[Upload] Error while uploading " + filePath + ": " + strconv.Itoa(resp.StatusCode))
		}
	}
	return err
}

// Creates a new file upload http request with optional extra params
func (f *Fetcher) Upload(uri string, params map[string]string, paramName, path string) (*http.Request, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	fileContents, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}
	fi, err := file.Stat()
	if err != nil {
		return nil, err
	}
	file.Close()

	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile(paramName, fi.Name())
	if err != nil {
		return nil, err
	}
	part.Write(fileContents)

	for key, val := range params {
		_ = writer.WriteField(key, val)
	}
	err = writer.Close()
	if err != nil {
		return nil, err
	}

	request, err := http.NewRequest("POST", f.BaseURL+uri, body)
	if err != nil {
		return request, nil
	}
	request.Header.Add("Content-Type", writer.FormDataContentType())
	return request, nil
}
