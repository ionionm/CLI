package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
)

var (
	bvid string
	url  string
)

const (
	getCidAPI      = "https://api.bilibili.com/x/player/pagelist?bvid=%v"
	getVideoUrlAPI = "https://api.bilibili.com/x/player/playurl?bvid=%v&cid=%v&qn=%v"
)

func init() {
	flag.StringVar(&bvid, "bvid", "", "The BVID needs you to provid")
	flag.StringVar(&url, "url", "", "The url you provided.")
	flag.Parse()
}

//每个视频对应一个Cid
//BiLibili Cid
type BLCid struct {
}

func main() {
	var err error
	if bvid == "" {
		bvid, err = extractBVID(url)
		if err != nil {
			panic(err)
		}
	}
	getCids(bvid)
}

func getCids(bvid string) []*BLCid {
	getCidUrl := fmt.Sprintf(getCidAPI, bvid)
	client := &http.Client{}
	req, err := http.NewRequest("GET", getCidUrl, nil)
	if err != nil {
		panic(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	io.Copy(os.Stdout, resp.Body)
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
