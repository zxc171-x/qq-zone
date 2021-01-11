package http

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"qq-zone/utils/helper"
	pbar "github.com/cheggaaa/pb/v3"
)

func Get(url string, msgs ...map[string]string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	headers := make(map[string]string)
	if len(msgs) > 0 {
		headers = msgs[0]
	}

	for key, val := range headers {
		req.Header.Set(key, val)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("The http request failed, the status code is: %s", resp.Status)
	}

	var buffer [512]byte
	result := bytes.NewBuffer(nil)
	for {
		n, err := resp.Body.Read(buffer[0:])
		result.Write(buffer[0:n])
		if err != nil && err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
	}
	return result.Bytes(), nil
}

/**
* 远程文件下载，支持断点续传，支持实时进度显示
* @param string uri 远程资源地址
* @param string target 调用时传入文件名，如果支持断点续传时当程序超时程序会自动调用该方法重新下载，此时传入的是文件句柄
* @param interface{} msgs 可变参数，参数顺序 0: retry int（下载失败后重试次数） 1：timeout int 超时，默认300s 2：progressbar bool 是否开启进度条，默认false
 */
func Download(uri string, target string, msgs ...interface{}) (map[string]interface{}, error) {
	filename := filepath.Base(target)
	entension := filepath.Ext(target)
	var targetDir string
	if entension != "" {
		filename = strings.Replace(filename, entension, "", 1)
		targetDir = filepath.Dir(target)
	} else {
		lasti := strings.LastIndex(target, "/")
		if lasti == -1 {
			return nil, fmt.Errorf("Not the correct file address")
		}
		targetDir = target[:lasti]
	}

	if (!helper.IsDir(targetDir)) {
		os.MkdirAll(targetDir, os.ModePerm)
	}

	retry := 0
	if len(msgs) > 0 {
		retry = msgs[0].(int)
	}

	timeout := 300
	if len(msgs) > 1 {
		timeout = msgs[1].(int)
	}

	progressbar := false
	if len(msgs) > 2 {
		progressbar = msgs[2].(bool)
	}

	hresp, err := http.Get(uri)
	if err != nil {
		if retry > 0 {
			return Download(uri, target, retry-1, timeout, progressbar)
		} else {
			return nil, fmt.Errorf("Failed to get response header, Error message → ", err.Error())
		}
	}
	hresp.Body.Close()

	contentRange := hresp.Header.Get("Content-Range")
	acceptRanges := hresp.Header.Get("Accept-Ranges")
	var ranges bool
	if contentRange != "" || acceptRanges == "bytes" {
		ranges = true
	}

	contentType := hresp.Header.Get("Content-Type")
	if contentType != "" && entension == "" {
		exts, err := mime.ExtensionsByType(contentType)
		if err == nil && len(exts) > 0 {
			entension = exts[0]
			filename = fmt.Sprintf("%s%s", filename, entension)
			target = fmt.Sprintf("%s/%s", targetDir, filename)
		}
	}

	var (
		size          int64 = 0
		contentLength int64 = hresp.ContentLength
	)

	if helper.IsFile(target) {
		if ranges {
			fileInfo, _ := os.Stat(target)
			if fileInfo != nil {
				size = fileInfo.Size()
			}
		} else {
			if err := os.Remove(target); err != nil {
				if retry > 0 {
					return Download(uri, target, retry-1, timeout, progressbar)
				} else {
					return nil, err
				}
			}
		}
	}

	res := make(map[string]interface{})
	if size == contentLength {
		res["filename"] = filename
		res["dir"] = targetDir
		res["path"] = target
		return res, nil
	}

	req, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		if retry > 0 {
			return Download(uri, target, retry-1, timeout, progressbar)
		} else {
			return nil, err
		}
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/78.0.3904.108 Safari/537.36")
	if ranges {
		req.Header.Set("Accept-Ranges", "bytes")
		req.Header.Set("Range", fmt.Sprintf("bytes=%v-", size))
	}

	client := &http.Client{
		Timeout: time.Second * time.Duration(timeout),
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		if retry > 0 {
			return Download(uri, target, retry-1, timeout, progressbar)
		} else {
			return nil, err
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if retry > 0 {
			return Download(uri, target, retry-1, timeout, progressbar)
		} else {
			return nil, fmt.Errorf("Http request was not successfully received and processed, status code is %v, status is %v", resp.StatusCode, resp.Status)
		}
	}

	file, err := os.OpenFile(target, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0666)
	if err != nil {
		if retry > 0 {
			return Download(uri, target, retry-1, timeout, progressbar)
		} else {
			return nil, err
		}
	}
	defer file.Close()

	if progressbar {
		reader := io.LimitReader(io.MultiReader(resp.Body), int64(resp.ContentLength))
		bar := pbar.Full.Start64(resp.ContentLength)
		barReader := bar.NewProxyReader(reader)
		_, err := io.Copy(file, barReader)
		bar.Finish()
		if err != nil {
			if retry > 0 {
				return Download(uri, target, retry-1, timeout, progressbar)
			} else {
				return nil, err
			}
		}
	} else {
		_, err = io.Copy(file, resp.Body)
		if err != nil {
			if retry > 0 {
				return Download(uri, target, retry-1, timeout, progressbar)
			} else {
				return nil, err
			}
		}
	}

	fi, _ := os.Stat(target)
	if fi != nil {
		size = fi.Size()
	}

	if contentLength != size {
		return nil, fmt.Errorf("The source file and the target file size are inconsistent")
	}

	res["filename"] = filename
	res["dir"] = targetDir
	res["path"] = target
	return res, nil
}