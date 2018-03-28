package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"sync"
)

const (
	Fmlink = "http://music.baidu.com/data/music/fmlink"
	Fmlist = "http://fm.baidu.com/dev/api/?tn=channellist"
)

type Songs struct {
	HashCode    string      `json:"hash_code"`
	ChannelId   string      `json:"channel_id"`
	ChannelName string      `json:"channel_name"`
	List        []List      `json:"list"`
	Results     interface{} `json:"results"`
	Status      int         `json:"status"`
}

type List struct {
	Id       int `json:"id"`
	Type     int `json:"type"`
	Method   int `json:"method"`
	FlowMark int `json:"flow_mark"`
}

func main() {
	if len(os.Args) <= 1 {
		fmt.Println("请输入歌曲列表.")
		return
	}
	fmt.Println("fetching msg from ", os.Args[1])

	nurl := fmt.Sprintf("http://fm.baidu.com/dev/api/?tn=playlist&format=json&id=%s", os.Args[1])
	//	nurl := fmt.Sprintf("http://fm.baidu.com/dev/api/?tn=playlist&format=json&id=%s", "public_xinqing_huankuai")
	response, err := DownloadString(nurl, nil)
	fmt.Println(string(response))
	if err != nil {
		fmt.Println("获取远程URL内容时出错：", err)
		return
	}

	var songs Songs
	err = json.Unmarshal(bytes.Trim(response, "\x00"), &songs)
	if err != nil {
		fmt.Println("反序列化JSON时出错:", err)
	}

	var path string
	if os.IsPathSeparator('\\') {
		path = "\\"
	} else {
		path = "/"
	}
	dir, _ := os.Getwd()
	dir = dir + path + songs.ChannelName + path
	if _, err := os.Stat(dir); err != nil {
		err = os.Mkdir(dir, os.ModePerm)
		if err != nil {
			fmt.Println("创建目录失败：", err)
			return
		}
	}

	wg := sync.WaitGroup{}
	for _, song := range songs.List {
		if song.Id == 0 {
			continue
		}
		wg.Add(1)
		go DownloadMusic(&wg, song.Id, dir)
	}

	wg.Wait()
	fmt.Println("Download Over.")
}

func DownloadString(remoteUrl string, queryValues url.Values) (body []byte, err error) {
	client := &http.Client{}
	body = nil
	uri, err := url.Parse(remoteUrl)
	if err != nil {
		return
	}
	if queryValues != nil {
		values := uri.Query()
		if values != nil {
			for k, v := range values {
				queryValues[k] = v
			}
		}
		uri.RawQuery = queryValues.Encode()
	}
	reqest, err := http.NewRequest("GET", uri.String(), nil)
	reqest.Header.Add("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	reqest.Header.Add("Accept-Encoding", "gzip, deflate")
	reqest.Header.Add("Accept-Language", "zh-cn,zh;q=0.8,en-us;q=0.5,en;q=0.3")
	reqest.Header.Add("Connection", "keep-alive")
	reqest.Header.Add("Host", uri.Host)
	reqest.Header.Add("Referer", uri.String())
	reqest.Header.Add("User-Agent", "Mozilla/5.0 (Windows NT 6.1; WOW64; rv:12.0) Gecko/20100101 Firefox/12.0")

	response, err := client.Do(reqest)
	defer response.Body.Close()
	if err != nil {
		return
	}

	if response.StatusCode == 200 {
		switch response.Header.Get("Content-Encoding") {
		case "gzip":
			reader, _ := gzip.NewReader(response.Body)
			for {
				buf := make([]byte, 1024)
				n, err := reader.Read(buf)

				if err != nil && err != io.EOF {
					panic(err)
				}

				if n == 0 {
					break
				}
				body = append(body, buf...)
			}
		default:
			body, _ = ioutil.ReadAll(response.Body)

		}
	}
	return
}

func DownloadMusic(wg *sync.WaitGroup, id int, path string) error {
	defer wg.Done()

	query := url.Values{}
	query.Set("songIds", fmt.Sprintf("%d", id))
	query.Set("type", "flac")
	res, err := DownloadString(Fmlink, query)
	fmt.Println("res:", string(res))
	if err != nil {
		fmt.Println("获取音乐文件时出错：", err)
		return fmt.Errorf("获取音乐文件时出错：", err)
	}

	var data map[string]interface{}
	err = json.Unmarshal(res, &data)
	if code, ok := data["errorCode"]; (ok && code.(float64) == 22005) || err != nil || code.(float64) == 22012 {
		fmt.Println("解析音乐文件时出错：", err)
		return fmt.Errorf("解析音乐文件时出错：", err)
	}

	songlink := data["data"].(map[string]interface{})["songList"].([]interface{})[0].(map[string]interface{})["songLink"].(string)
	r := []rune(songlink)
	if len(r) < 10 {
		fmt.Println("没有无损音乐地址: %v", id)
		return fmt.Errorf("没有无损音乐地址: %v", id)
	}

	songname := data["data"].(map[string]interface{})["songList"].([]interface{})[0].(map[string]interface{})["songName"].(string)
	artistName := data["data"].(map[string]interface{})["songList"].([]interface{})[0].(map[string]interface{})["artistName"].(string)
	filename := path + songname + "-" + artistName + ".flac"

	fmt.Println("正在下载 ", songname, " ......")
	songRes, err := http.Get(songlink)
	if err != nil {
		fmt.Println("下载文件时出错：", songlink)
		return fmt.Errorf("下载文件时出错：", songlink)
	}

	songFile, err := os.Create(filename)
	written, err := io.Copy(songFile, songRes.Body)
	if err != nil {
		fmt.Println("保存音乐文件时出错：", err)
		return fmt.Errorf("保存音乐文件时出错：", err)
	}
	fmt.Println(songname, "下载完成,文件大小：", fmt.Sprintf("%.2f", (float64(written)/(1024*1024))), "MB")

	return nil
}
