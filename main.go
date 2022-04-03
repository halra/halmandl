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
	junkSize        int
	ConcurrentParts int64
	PercDone        float32
	LastMeassured   int64
	BytesPerSecond  float32
}

func CDownload(dir string, url string) {

	stats := &FileStats{} // TODO  build this at the end in a one liner

	//Params
	concurrentParts := int64(6) // allow big files ...
	junkSize := 4194304         //each junk in bytes
	//End Params
	stats.ConcurrentParts = concurrentParts
	stats.junkSize = junkSize

	var wg sync.WaitGroup

	_, file := path.Split(url) // filename
	filepath := path.Join(dir, file)
	stats.Filename = file
	stats.TargetDirectory = filepath

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

	res, _ := http.Head(url) // fetch required headers, we decide on the header if we use junks
	length, _ := strconv.ParseInt(res.Header.Get("Content-Length"), 10, 64)
	acceptRange := res.Header.Get("Accept-Ranges") // check if we can use a concurrent strategie
	fmt.Println("Accept-Ranges", acceptRange)
	limit := int64(1)
	if strings.Contains(acceptRange, "bytes") {
		limit = length / int64(junkSize) // split in junks of n bytes
		if limit == 0 {                  // handle small files, files < junksize
			limit = 1
		}
	}
	lenJunk := length / limit // Bytes for each Go-routine
	diff := length % limit    //  the remaining for the last junk

	//stat HELPER
	stats.Parts = int(limit)
	stats.StartedAt = time.Now()
	stats.Size = length

	//END stat HELPER

	guard := make(chan struct{}, concurrentParts) // semaphore for concurrent requests
	defer func() {
		close(guard)
	}()

	//this kinda fails on junks ..., consider whole time bytes/time -> first meassure
	stats.LastMeassured = time.Now().UnixNano()

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

			stats.Junk = i
			client := &http.Client{}
			req, _ := http.NewRequest("GET", url, nil)
			range_header := "bytes=" + strconv.FormatInt(min, 10) + "-" + strconv.FormatInt(max, 10) // add header for junk size
			req.Header.Add("Range", range_header)
			resp, err := client.Do(req)

			if err != nil {
				fmt.Println(err)
			}

			defer resp.Body.Close()

			//just download normaly
			if limit == 1 {
				_, err = io.Copy(f, resp.Body)
				if err != nil {
					fmt.Println(err)
				}
			} else { // download the junks of the file
				var reader []byte
				reader, _ = ioutil.ReadAll(resp.Body)
				stats.Transfered = stats.Transfered + int64(len(reader))
				go test(stats)
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
func test(f *FileStats) {
	//fmt.Println(f)
	divider := (time.Now().UnixNano() - f.LastMeassured) / time.Second.Nanoseconds()
	if divider == 0 {
		divider = 1
	}
	f.BytesPerSecond = float32(int64(f.junkSize)) / float32(divider)
	f.PercDone = float32(f.Transfered) / float32(f.Size)
	//fmt.Printf("%+v\n", f.BytesPerSecond)
	f.LastMeassured = time.Now().UnixNano()
}
