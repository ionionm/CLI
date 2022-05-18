# Bilibili Downloader



## Extract BVID

extract BVID from a specifil URL Given by terminal.

such as `"https://www.bilibili.com/video/BV1xK4y1d7oL?spm_id_from=333.337.search-card.all.click"`

USE regexp **<span style="color:red">`/?(BV\w+)[/?]?`</span>** to findSubMatch, will get `/BV1xK4y1d7oL?` & `BV1xK4y1d7oL`

## Build Video struct

### Get CID

Cid is the abstraction of a video, a video has a unique cid related.

We can get cid from API `"https://api.bilibili.com/x/player/pagelist?bvid=%v"`

```go
func rawGetURL(URL string, data interface{}, f func(*http.Request)) error {
	client := &http.Client{}
	req, err := http.NewRequest("GET", URL, nil)
	if err != nil {
		return err
	}
	if f != nil {
		f(req)
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	err = json.NewDecoder(resp.Body).Decode(data)
	if err != nil {
		return err
	}
	return nil
}

func getVideos(bvid string) []*Video {
	getCidUrl := fmt.Sprintf(getCidAPI, bvid)
	var blCid BLCid
	err := rawGetURL(getCidUrl, &blCid, nil)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%+v\n", blCid)
	var videos []*Video
	for _, data := range blCid.Data {
		v := Video{}
		v.Cid = strconv.FormatInt(data.Cid, 10)
		v.Title = data.Part
		v.Bvid = bvid
		v.QN = qn
		v.getPlayURLs()
		videos = append(videos, &v)
	}
	return videos
}
```

### Get video’s playURL

If the quality of the video > 720P, then you need cookie carried when search palyURL.

```go
func (v *Video) getPlayURLs() {
	getPlayURL := fmt.Sprintf(getVideoUrlAPI, v.Bvid, v.Cid, v.QN)
	log.Printf("url: %v\n", getPlayURL)
	pl := struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Quality int `json:"quality"`
			Durl    []struct {
				Order     int    `json:"order"`
				URL       string `json:"url"`
				BackupURL string `json:"backup_url"`
			} `json:"durl"`
		} `json:"data"`
	}{}
	rawGetURL(getPlayURL, &pl, setCookie)
	fmt.Printf("%+v\n", pl)
	for _, d := range pl.Data.Durl {
		v.PlayURLs = append(v.PlayURLs, d.URL)
	}
}
```

## Download Videos

### Define customer downloader

The customer downloader is the wrapper of resp.Body, the only purpose is to add a visuable progress level when downloading.

```go
func (v *Video) download(dir string) {
	client := &http.Client{}
	for _, URL := range v.PlayURLs {
		u, err := url.Parse(URL)
		if err != nil {
			panic(err)
		}
		req, _ := http.NewRequest("GET", URL, nil)
		setUserAgent(req)
		setCookie(req)
		req.Header.Set("Accept", "*/*")
		req.Header.Set("Accept-Language", "en-US,en;q=0.5")
		req.Header.Set("Accept-Encoding", "gzip, deflate, br")
		req.Header.Set("Range", "bytes=0-")                               // Range 的值要为 bytes=0- 才能下载完整视频
		req.Header.Set("Referer", "https://www.bilibili.com/video/"+bvid) // 必需添加
		req.Header.Set("Origin", "https://www.bilibili.com")
		req.Header.Set("Connection", "keep-alive")

		resp, err := client.Do(req)
		if err != nil {
			panic(err)
		}
		defer resp.Body.Close()
		dir = filepath.Join(dir, path.Base(u.Path))
		f, err := os.OpenFile(dir, os.O_RDWR|os.O_CREATE, 0755)
		if err != nil {
			panic(err)
		}
		d := &downloader{
			resp.Body,
			resp.ContentLength,
			0,
		}
		io.Copy(f, d)
		fmt.Println("")
	}
}

type downloader struct {
	io.Reader
	Total   int64
	Current int64
}

func (d *downloader) Read(p []byte) (n int, err error) {
	n, err = d.Reader.Read(p)
	d.Current += int64(n)
	fmt.Printf("\rprogress: %.2f%%", float64(d.Current)*100.0/float64(d.Total))
	return
}
```

## The Whole Code

```go
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	bvid        string
	dir         string
	URL         string
	qn          int
	_qn         string
	sessionData string
)

const (
	getCidAPI      = "https://api.bilibili.com/x/player/pagelist?bvid=%v"
	getVideoUrlAPI = "https://api.bilibili.com/x/player/playurl?bvid=%v&cid=%v&qn=%v&fourk=1"
)

var qnMap = map[string]struct {
	QN         int
	NeedCookie bool
	Detail     string
}{
	"4K":      {120, true, "超清 4K"},
	"1080P60": {116, true, "高清1080P60"},
	"1080P+":  {112, true, "高清1080P+"},
	"1080P":   {80, true, "1080P"},
	"720P60":  {74, true, "高清720P60"},
	"720P":    {64, false, "高清720P"},
	"480P":    {32, false, "清晰480P"},
	"360P":    {16, false, "流畅360P"},
}

func init() {
	flag.StringVar(&bvid, "b", "", "The BVID needs you to provid")
	flag.StringVar(&dir, "d", "./", "The directory you want to save the video downloaded.")
	flag.StringVar(&URL, "u", "", "The url you provided.")
	flag.StringVar(&_qn, "q", "720P", "The quality of the video you want.")
	flag.StringVar(&sessionData, "s", "614467e6%2C1667359924%2Cf5560*51", "value of your bilibili cookie[\"SESSDATA\"]")
	flag.Parse()
}

type Dimension struct {
	Width  int `json:"width"`
	Height int `json:"height"`
	Rotate int `json:"rotate"`
}

type CidData struct {
	Cid      int64     `json:"cid"`
	Page     int       `json:"page"`
	Part     string    `json:"part"`
	Duration int       `json:"duration"`
	Vid      string    `json:"vid"`
	Dimen    Dimension `json:"Dimension"`
}

//每个视频对应一个Cid
//BiLibili Cid
type BLCid struct {
	Code    int       `json:"code"`
	Message string    `json:"message"`
	Data    []CidData `json:"data"`
}

type downloader struct {
	io.Reader
	Total   int64
	Current int64
}

func (d *downloader) Read(p []byte) (n int, err error) {
	n, err = d.Reader.Read(p)
	d.Current += int64(n)
	fmt.Printf("\rprogress: %.2f%%", float64(d.Current)*100.0/float64(d.Total))
	return
}

type Video struct {
	Bvid     string
	Cid      string
	Title    string
	QN       int
	PlayURLs []string
}

func main() {
	var err error
	_qn = strings.ToUpper(_qn)
	if v, ok := qnMap[_qn]; ok {
		qn = v.QN
	}
	if bvid == "" {
		bvid, err = extractBVID(URL)
		if err != nil {
			panic(err)
		}
	}
	videos := getVideos(bvid)
	for _, v := range videos {
		v.download(dir)
	}
}

func getVideos(bvid string) []*Video {
	getCidUrl := fmt.Sprintf(getCidAPI, bvid)
	var blCid BLCid
	err := rawGetURL(getCidUrl, &blCid, nil)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%+v\n", blCid)
	var videos []*Video
	for _, data := range blCid.Data {
		v := Video{}
		v.Cid = strconv.FormatInt(data.Cid, 10)
		v.Title = data.Part
		v.Bvid = bvid
		v.QN = qn
		v.getPlayURLs()
		videos = append(videos, &v)
	}
	return videos
}

func (v *Video) download(dir string) {
	client := &http.Client{}
	for _, URL := range v.PlayURLs {
		u, err := url.Parse(URL)
		if err != nil {
			panic(err)
		}
		req, _ := http.NewRequest("GET", URL, nil)
		setUserAgent(req)
		setCookie(req)
		req.Header.Set("Accept", "*/*")
		req.Header.Set("Accept-Language", "en-US,en;q=0.5")
		req.Header.Set("Accept-Encoding", "gzip, deflate, br")
		req.Header.Set("Range", "bytes=0-")                               // Range 的值要为 bytes=0- 才能下载完整视频
		req.Header.Set("Referer", "https://www.bilibili.com/video/"+bvid) // 必需添加
		req.Header.Set("Origin", "https://www.bilibili.com")
		req.Header.Set("Connection", "keep-alive")

		resp, err := client.Do(req)
		if err != nil {
			panic(err)
		}
		defer resp.Body.Close()
		dir = filepath.Join(dir, path.Base(u.Path))
		f, err := os.OpenFile(dir, os.O_RDWR|os.O_CREATE, 0755)
		if err != nil {
			panic(err)
		}
		d := &downloader{
			resp.Body,
			resp.ContentLength,
			0,
		}
		io.Copy(f, d)
		fmt.Println("")
	}
}

func setUserAgent(req *http.Request) {
	// User-Agent会告诉网站服务器，访问者是通过什么工具来请求的，如果是爬虫请求，一般会拒绝，如果是用户浏览器，就会应答。
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/101.0.4951.64 Safari/537.36")
}

func setCookie(req *http.Request) {
	cookie := http.Cookie{Name: "SESSDATA", Value: sessionData, Expires: time.Now().Add(30 * 24 * 60 * 60 * time.Second)}
	log.Printf("got bilibili cookie, SESSDATA:%v", sessionData)
	req.AddCookie(&cookie)
}

func (v *Video) getPlayURLs() {
	getPlayURL := fmt.Sprintf(getVideoUrlAPI, v.Bvid, v.Cid, v.QN)
	log.Printf("url: %v\n", getPlayURL)
	pl := struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Quality int `json:"quality"`
			Durl    []struct {
				Order     int    `json:"order"`
				URL       string `json:"url"`
				BackupURL string `json:"backup_url"`
			} `json:"durl"`
		} `json:"data"`
	}{}
	rawGetURL(getPlayURL, &pl, setCookie)
	fmt.Printf("%+v\n", pl)
	for _, d := range pl.Data.Durl {
		v.PlayURLs = append(v.PlayURLs, d.URL)
	}
}

func rawGetURL(URL string, data interface{}, f func(*http.Request)) error {
	client := &http.Client{}
	req, err := http.NewRequest("GET", URL, nil)
	if err != nil {
		return err
	}
	if f != nil {
		f(req)
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	err = json.NewDecoder(resp.Body).Decode(data)
	if err != nil {
		return err
	}
	return nil
}

//从URL中提取BVID
func extractBVID(url string) (string, error) {
	if url == "" {
		return "", errors.New("Invalid url provided: " + url)
	}
	regexp := regexp.MustCompile(`/?(BV\w+)[/?]?`)
	params := regexp.FindStringSubmatch(url)
	if len(params) < 1 {
		return "", errors.New("Invalid url provided: " + url)
	}
	return params[1], nil
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
```

## Example

```bash
go run main.go -u "https://www.bilibili.com/video/BV1xK4y1d7oL?spm_id_from=333.337.search-card.all.click" -q 4K
```

