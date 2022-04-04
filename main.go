package halmandl

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
)

type FileStats struct {
	Transfered      int64
	Filename        string
	Size            int64
	TargetDirectory string
	StartedAt       time.Time
	Junk            int64
	Parts           int
	junkSize        int64
	ConcurrentParts int64
	PercDone        float32
	LastMeassured   int64
	BytesPerSecond  float32
	CompletedJunks  int64
}

type Options struct {
	JunkSize        int64
	ConcurrentParts int64
	UseStats        bool
}

func CDownload(dir string, url string, options Options) {

	//Params
	if options.ConcurrentParts == 0 {
		options.ConcurrentParts = int64(6) // allow big files ...
	}
	if options.JunkSize == 0 {
		options.JunkSize = int64(4194304) // allow big files ...
	}
	//End Params

	var wg sync.WaitGroup

	_, file := path.Split(url) // filename
	filepath := path.Join(dir, file)

	err := os.MkdirAll(dir, os.ModePerm) // create path if not exist
	if err != nil {
		fmt.Println(err)
	}

	f, err := os.Create(filepath) // create the file
	if err != nil {
		fmt.Println(err)
		return
	}
	defer f.Close()
	length := int64(1)
	limit := int64(1)
	//TODO some servers wont respond here, we could peek with a range ret to read the headers
	res, err := http.Head(url) // fetch required headers, we decide on the header if we use junks
	if err == nil {            // Only do if header is present
		//fmt.Println(err)
		contentLengthHeader := res.Header.Get("Content-Length")
		if contentLengthHeader != "" {
			length, _ = strconv.ParseInt(res.Header.Get("Content-Length"), 10, 64)
		}
		acceptRange := res.Header.Get("Accept-Ranges") // check if we can use a concurrent strategie
		fmt.Println("Accept-Ranges", acceptRange)
		if strings.Contains(acceptRange, "bytes") {
			limit = length / int64(options.JunkSize) // split in junks of n bytes
			if limit == 0 {                          // handle small files, files < junksize
				limit = 1
			}
		}
	}
	lenJunk := length / limit                             // Bytes for each Go-routine
	diff := length % limit                                //  the remaining for the last junk
	guard := make(chan struct{}, options.ConcurrentParts) // semaphore for concurrent requests
	defer func() {
		close(guard)
	}()

	//Init stats
	stats := &FileStats{Transfered: 0, Filename: file, Parts: int(limit), StartedAt: time.Now(), Size: length,
		TargetDirectory: filepath, ConcurrentParts: options.ConcurrentParts, junkSize: options.JunkSize, CompletedJunks: int64(0)}
	for i := int64(0); i < limit; i++ {

		guard <- struct{}{} // add semaphore
		wg.Add(1)
		min := lenJunk * i       // byte range
		max := lenJunk * (i + 1) // byte range

		if i == limit-1 {
			max += diff // Add the remaining bytes in the last request
		}

		go func(min int64, max int64, i int64) {
			defer func(guard chan struct{}) {
				<-guard // release semaphore
			}(guard)
			defer func(wg *sync.WaitGroup) {
				// release wg semaphore
				wg.Done()
			}(&wg)

			stats.Junk = i // possible race bugs
			client := &http.Client{}
			req, _ := http.NewRequest("GET", url, nil)
			if limit > 1 { // only had
				rangeHeader := "bytes=" + strconv.FormatInt(min, 10) + "-" + strconv.FormatInt(max, 10) // add header for junk size
				req.Header.Add("Range", rangeHeader)
			}
			resp, err := client.Do(req)

			if err != nil {
				fmt.Println(err)
			}

			defer resp.Body.Close()

			//just download normaly
			if limit == 1 {
				if options.UseStats {
					go doStats(stats, false)
				}
				_, err = io.Copy(f, resp.Body)
				if err != nil {
					fmt.Println(err)
				}
			} else { // download the junks of the file
				var reader []byte
				reader, _ = ioutil.ReadAll(resp.Body)
				stats.Transfered = stats.Transfered + int64(len(reader)) // possible race bugs
				if options.UseStats {
					go doStats(stats, true)
				}
				_, err = f.WriteAt(reader, int64(min))
				if err != nil {
					fmt.Println(err)
				}
			}

		}(min, max, i)
	}
	wg.Wait()
}

// TODO write a stat struct and pass as channel
func doStats(f *FileStats, hasJunks bool) {
	//fmt.Println(f)
	divider := ((time.Now().UnixMilli() / 1000) - f.StartedAt.Unix())
	if divider == 0 {
		divider = 1
	}
	f.BytesPerSecond = float32(int64(f.Transfered)) / float32(divider) // quite naive but can work,also dosn't meassure single junks
	f.PercDone = float32(f.Transfered) / float32(f.Size)
	fmt.Printf("%+v\n", f)
	f.LastMeassured = time.Now().UnixNano()
}
