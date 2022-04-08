package halmandl

import (
	"crypto/sha1"
	"encoding/hex"
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

	op := Options{JunkSize: 4194300, ConcurrentParts: 10, UseStats: true}

	if mockString == "" {
		mockString = RandStringRunes(123456789)
	}

	origHash := getSha1([]byte(mockString))
	Download("./", testServer.URL+"/f1.txt", op)
	data, _ := ioutil.ReadFile("./f1.txt")
	doHash := getSha1(data)
	if origHash != doHash {
		t.Logf("TestCDownloadWithRange failed with missmatched hash\n")
		t.Fail()
	}
}

func TestCDownloadWithRangeBrokenServer(t *testing.T) {
	if mockString == "" {
		mockString = RandStringRunes(123456789)
	}
	op := Options{JunkSize: 219400, ConcurrentParts: 10, UseStats: true}
	origHash := getSha1([]byte(mockString))
	Download("./", testServerBroken.URL+"/f2.txt", op)
	data, _ := ioutil.ReadFile("./f2.txt")
	doHash := getSha1(data)
	if origHash != doHash {
		t.Logf("TestCDownloadWithRangeBrokenServer failed with matching hash\n")
		t.Fail()
	}
}

func TestCDownloadNoRange(t *testing.T) {
	op := Options{JunkSize: 4194300, ConcurrentParts: 5, UseStats: true}
	mockString = RandStringRunes(123456789)
	origHash := getSha1([]byte(mockString))
	Download("./", testServerNoRange.URL+"/f3.txt", op)
	data, _ := ioutil.ReadFile("./f3.txt")
	doHash := getSha1(data)
	if origHash != doHash {
		t.Logf("TestCDownloadWithRange failed\n")
		t.Fail()
	}
}

func TestCDownloadSmallFiles(t *testing.T) {
	op := Options{JunkSize: 5, ConcurrentParts: 5, UseStats: true}
	mockString = RandStringRunes(1)
	origHash := getSha1([]byte(mockString))
	Download("./", testServer.URL+"/f4.txt", op)
	data, _ := ioutil.ReadFile("./f4.txt")
	doHash := getSha1(data)
	if origHash != doHash {
		t.Logf("TestCDownloadWithRangeBrokenServer failed with missmatched hash\n")
		t.Fail()
	}
}

//Test servers

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

	min := 0
	max := 100
	rand := rand.Intn(max-min) + min
	if len(splitted) == 2 {
		min, _ := strconv.ParseInt(splitted[0], 10, 64)
		if rand > 50 {
			//fmt.Println("rand made dl BROKEN")
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
