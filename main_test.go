package halmandl

import (
	"crypto/sha1"
	"encoding/hex"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestFoo(t *testing.T) {

	downloader := NewDownloader()
	downloader.Options.ConcurrentParts = 10
	downloader.Options.UseStats = true

	downloader.Download("C:\\Users\\ra\\Downloads\\ytDl", "https://ia802508.us.archive.org/16/items/microsoft_xbox360_s_part1/Silent%20Hill%20-%20Homecoming%20%28Germany%29%20%28En%2CFr%2CDe%2CEs%2CIt%29.zip")

}

var mockString string

func TestCDownloadWithRange(t *testing.T) {

	if mockString == "" {
		mockString = RandStringRunes(123456789)
	}

	downloader := NewDownloader()
	downloader.Options.ConcurrentParts = 10
	downloader.Options.JunkSize = 4194300
	downloader.Options.UseStats = true

	origHash := getSha1([]byte(mockString))
	downloader.Download("./", testServer.URL+"/f1.txt")
	data, _ := os.ReadFile("./f1.txt")
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
	origHash := getSha1([]byte(mockString))
	downloader := NewDownloader()
	downloader.Options.ConcurrentParts = 10
	downloader.Options.JunkSize = 219400
	downloader.Options.UseStats = true

	downloader.Download("./", testServerBroken.URL+"/f2.txt")
	data, _ := os.ReadFile("./f2.txt")
	doHash := getSha1(data)
	if origHash != doHash {
		t.Logf("TestCDownloadWithRangeBrokenServer failed with matching hash\n")
		t.Fail()
	}
}

func TestCDownloadNoRange(t *testing.T) {
	mockString = RandStringRunes(123456789)
	origHash := getSha1([]byte(mockString))

	downloader := NewDownloader()
	downloader.Options.ConcurrentParts = 5
	downloader.Options.JunkSize = 4194300
	downloader.Options.UseStats = true

	downloader.Download("./", testServerNoRange.URL+"/f3.txt")
	data, _ := os.ReadFile("./f3.txt")
	doHash := getSha1(data)
	if origHash != doHash {
		t.Logf("TestCDownloadWithRange failed\n")
		t.Fail()
	}
}

func TestCDownloadSmallFiles(t *testing.T) {
	mockString = RandStringRunes(1)
	origHash := getSha1([]byte(mockString))

	downloader := NewDownloader()
	downloader.Options.ConcurrentParts = 5
	downloader.Options.JunkSize = 5
	downloader.Options.UseStats = true

	downloader.Download("./", testServer.URL+"/f4.txt")
	data, _ := os.ReadFile("./f4.txt")
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
