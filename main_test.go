package halmandl

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

//TODO calc hash
var mockString string

func TestCDownloadWithRange(t *testing.T) {

	op := Options{JunkSize: 4194304, ConcurrentParts: 10, UseStats: true}

	mockString = RandStringRunes(123456789)
	origHash := getSha1([]byte(mockString))
	fmt.Println(origHash)
	CDownload("/", testServer.URL+"/foo.txt", op)
	data, _ := ioutil.ReadFile("/\\foo.txt")
	doHash := getSha1(data)
	fmt.Println(doHash)
	if !strings.EqualFold(origHash, doHash) {
		t.Logf("TestCDownloadWithRange failed\n")
		t.Fail()
	}
}

func TestCDownloadWithRangeBrokenServer(t *testing.T) {

	op := Options{JunkSize: 4194304, ConcurrentParts: 10, UseStats: true}

	mockString = RandStringRunes(123456789)
	origHash := getSha1([]byte(mockString))
	fmt.Println(origHash)
	CDownload("/", testServerBroken.URL+"/foo2.txt", op)
	data, _ := ioutil.ReadFile("/\\foo2.txt")
	doHash := getSha1(data)
	fmt.Println(doHash)
	if strings.EqualFold(origHash, doHash) {
		t.Logf("TestCDownloadWithRange failed\n")
		t.Fail()
	}
}

func TestCDownloadNoRange(t *testing.T) {
	op := Options{JunkSize: 5, ConcurrentParts: 5, UseStats: true}
	mockString = RandStringRunes(123456789)
	origHash := getSha1([]byte(mockString))
	fmt.Println(origHash)
	CDownload("/", testServerNoRange.URL+"/bar.txt", op)
	data, _ := ioutil.ReadFile("/\\bar.txt")
	doHash := getSha1(data)
	fmt.Println(doHash)
}

func TestCDownloadSmallFiles(t *testing.T) {
	op := Options{JunkSize: 5, ConcurrentParts: 5, UseStats: true}
	mockString = RandStringRunes(1)
	CDownload("/", testServer.URL+"/foo1.txt", op)
}

var testServer = httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
	ra := req.Header.Get("Range")
	trimmed := strings.TrimPrefix(ra, "bytes=")
	splitted := strings.Split(trimmed, "-")
	res.Header().Add("Content-Length", strconv.Itoa(len(mockString)))
	res.Header().Add("Accept-Ranges", "bytes")

	if len(splitted) == 2 {
		min, _ := strconv.ParseInt(splitted[0], 10, 64)
		max, _ := strconv.ParseInt(splitted[1], 10, 64)
		res.Write([]byte(mockString)[min:max])
	} else {
		res.Write([]byte(mockString))
	}

}))

var testServerBroken = httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
	ra := req.Header.Get("Range")
	trimmed := strings.TrimPrefix(ra, "bytes=")
	splitted := strings.Split(trimmed, "-")
	res.Header().Add("Content-Length", strconv.Itoa(len(mockString)))
	res.Header().Add("Accept-Ranges", "bytes")

	if len(splitted) == 2 {
		min, _ := strconv.ParseInt(splitted[0], 10, 64)
		if min > 4194304 && min < 4194304*3 {
			return
		}
		max, _ := strconv.ParseInt(splitted[1], 10, 64)
		res.Write([]byte(mockString)[min:max])
	} else {
		res.Write([]byte(mockString))
	}

}))

var testServerNoRange = httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
	res.Header().Add("Content-Length", strconv.Itoa(len(mockString)))
	res.Write([]byte(mockString))
}))

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func RandStringRunes(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

func getSha1(data []byte) string {
	h := sha1.New()
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}
