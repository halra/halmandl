package halmandl

import (
	"bytes"
	"fmt"
	"io"
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
	JunkSize        int64
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
		TargetDirectory: filepath, ConcurrentParts: options.ConcurrentParts, JunkSize: options.JunkSize, CompletedJunks: int64(0)}

	//stats:
	type Result struct {
		bytes *bytes.Buffer
		id    int64
	}
	startTransaction := make(chan Result)
	endTransaction := make(chan int64) // TODO can be max cocnurrent
	dieStats := make(chan bool)
	defer func() {
		dieStats <- true
	}()

	//Start stats listener
	go func(f *FileStats) {
		holder := map[int64]Result{}
		die := false
		for {
			//TODO select incomming bytes

			select {
			case reader := <-startTransaction:
				holder[reader.id] = reader
			case i := <-endTransaction:
				f.Transfered = f.Transfered + int64(holder[i].bytes.Len())
				delete(holder, i)
			case <-time.After(time.Second * 1):
				divider := ((time.Now().UnixMilli() / 1000) - f.StartedAt.Unix())
				if divider == 0 {
					divider = 1
				}
				counter := 0
				for _, v := range holder {
					counter = counter + v.bytes.Len()
				}

				transveredSum := int64(counter) + f.Transfered
				f.BytesPerSecond = float32(int64(transveredSum)) / float32(divider) // sums the whole time ... better would be to meassure each frame
				f.PercDone = float32(transveredSum) / float32(f.Size)
				//fmt.Printf("%+v\n", f)
				f.LastMeassured = time.Now().UnixNano()
				fmt.Println("StartedAt:", f.StartedAt)
				fmt.Println("Filename:", f.Filename, "junkssize:", f.JunkSize)
				fmt.Printf("Junks: %v / %v \n", f.CompletedJunks, f.Parts)
				fmt.Printf("Progress: %v / %v\n", transveredSum, f.Size)
				fmt.Printf("Percent completed: %.2f\n", f.PercDone*100)
				fmt.Printf("Bps: %.2f \n", f.BytesPerSecond)

			case die = <-dieStats:
			}

			if die {
				close(dieStats)
				close(startTransaction)
				close(endTransaction)
				break
			}
		}

	}(stats)

	//end stats

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
				stats.CompletedJunks += 1
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

			reader := bytes.NewBuffer(nil)
			if options.UseStats {
				startTransaction <- Result{id: i, bytes: reader}
			}
			_, err = io.Copy(reader, resp.Body)
			_, err = f.WriteAt(reader.Bytes(), int64(min))
			endTransaction <- i
			if err != nil {
				fmt.Println(err)
			}

		}(min, max, i)
	}
	wg.Wait()

}
