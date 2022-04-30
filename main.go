package halmandl

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Parts struct {
	Min int64
	Max int64
	Idx int64
}

type Helper struct {
	Parts        []Parts
	Comleted     []int64
	Failed       []int64
	AllComplete  bool
	PartsLen     int64
	CompletedSum int64
	FailedSum    int64
	Options      Options
}

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
	MaxTries        int
}

func Download(dir string, url string, options Options) {
	maxTries := max(10, int64(options.MaxTries))
	for maxTries > 0 {
		if CDownload(dir, url, options) {
			return
		}
		maxTries -= 1
	}
}

func DownloadStandard(dir string, url string) {

}

func CDownload(dir string, inUrl string, options Options) bool {

	//unescape url path
	inUrl, _ = url.PathUnescape(inUrl)
	inUrl, _ = url.QueryUnescape(inUrl)

	//Params
	options.ConcurrentParts = max(1, options.ConcurrentParts)
	options.JunkSize = max(4194304, options.JunkSize)
	//End Params

	var wg sync.WaitGroup

	_, file := path.Split(inUrl) // filename
	filepath := path.Join(dir, file)

	err := os.MkdirAll(dir, os.ModePerm) // create path if not exist
	if err != nil {
		fmt.Println(err)
	}

	//create downloadfile
	f, err := os.OpenFile(filepath, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		fmt.Println(err)
		return false
	}
	defer f.Close()

	//Load the config file and resume the download if we already have parts
	filewatcherFilename := filepath + "halmandl"
	fileWatcherFile, _ := os.OpenFile(filewatcherFilename, os.O_RDWR|os.O_CREATE, 0666)
	defer fileWatcherFile.Close()
	jsonParser := json.NewDecoder(fileWatcherFile)

	fileWatcher := Helper{}
	//Set values from last dl if we find any
	if err = jsonParser.Decode(&fileWatcher); err != nil {
		fileWatcher.Options = options
	} else {
		options = fileWatcher.Options
	}
	//writeFileHelperToDir(fileWatcher, filewatcherFilename)

	length := int64(1)
	limit := int64(1)
	res, err := http.Head(inUrl) // fetch required headers, we decide on the header if we use junks
	if err == nil {              // Only do if header is present
		contentLengthHeader := res.Header.Get("Content-Length")
		if contentLengthHeader != "" {
			length, _ = strconv.ParseInt(res.Header.Get("Content-Length"), 10, 64)
		}
		acceptRange := res.Header.Get("Accept-Ranges") // check if we can use a concurrent strategie
		//fmt.Println("Accept-Ranges", acceptRange)
		if strings.Contains(acceptRange, "bytes") {
			limit = length / int64(options.JunkSize) // split in junks of n bytes
			if limit == 0 {                          // handle small files, files < junksize
				limit = 1
			}
		}
	}
	//Fill structior if nil
	if fileWatcher.Parts == nil {
		fileWatcher.Parts = make([]Parts, limit)
		fileWatcher.Comleted = make([]int64, limit)
		fileWatcher.Failed = make([]int64, limit)
	}

	lenJunk := length / limit                             // Bytes for each Go-routine
	diff := length % limit                                //  the remaining for the last junk
	guard := make(chan struct{}, options.ConcurrentParts) // semaphore for concurrent requests
	defer func() {
		close(guard)
	}()

	//Init stats
	stats := &FileStats{Transfered: 0, Filename: file, Parts: int(limit), StartedAt: time.Now(), Size: length,
		TargetDirectory: filepath, ConcurrentParts: options.ConcurrentParts, JunkSize: options.JunkSize, CompletedJunks: fileWatcher.CompletedSum}

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
			select {
			case reader := <-startTransaction:
				holder[reader.id] = reader
			case i := <-endTransaction:
				if holder[i].bytes != nil {
					f.Transfered = f.Transfered + int64(holder[i].bytes.Len())
				}
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
				f.LastMeassured = time.Now().UnixNano()
				fmt.Printf("\rFilename: %v p: %v / %v c: %.2f %% Junks: %v / %v Bps: %v                               ", f.Filename,
					transveredSum, f.Size, f.PercDone*100, f.CompletedJunks, f.Parts, f.BytesPerSecond)
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
		if fileWatcher.Comleted[i] == 1 {
			continue
		}
		min := lenJunk * i       // byte range
		max := lenJunk * (i + 1) // byte range

		if i == limit-1 {
			max += diff // Add the remaining bytes in the last request
		}
		fileWatcher.Parts[i].Min = min
		fileWatcher.Parts[i].Max = max
		fileWatcher.Parts[i].Idx = i
	}

	for j, p := range fileWatcher.Parts {

		guard <- struct{}{} // add semaphore
		wg.Add(1)
		min := p.Min
		max := p.Max

		if fileWatcher.Comleted[fileWatcher.Parts[j].Idx] == 1 {
			wg.Done()
			<-guard
			continue
		}

		go func(min int64, max int64, i int64) {
			defer func(guard chan struct{}) {
				stats.CompletedJunks += 1
				<-guard // release semaphore
			}(guard)
			defer func(wg *sync.WaitGroup) {
				// release wg semaphore
				fileWatcher.PartsLen = int64(len(fileWatcher.Parts))
				writeFileHelperToDir(fileWatcher, filewatcherFilename)
				wg.Done()
			}(&wg)

			stats.Junk = i
			client := &http.Client{}
			req, _ := http.NewRequest("GET", inUrl, nil)
			if limit > 1 {
				rangeHeader := "bytes=" + strconv.FormatInt(min, 10) + "-" + strconv.FormatInt(max, 10) // add header for junk size
				req.Header.Add("Range", rangeHeader)
			}
			resp, err := client.Do(req)
			if err != nil {
				fmt.Println(err)
			}
			defer resp.Body.Close()

			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				fileWatcher.Failed[i] = 1
				fileWatcher.FailedSum += 1
				return
			}

			//fmt.Println(resp.StatusCode)

			reader := bytes.NewBuffer(nil)
			if options.UseStats {
				startTransaction <- Result{id: i, bytes: reader}
			}
			_, err = io.Copy(reader, resp.Body)
			written, err := f.WriteAt(reader.Bytes(), int64(min))
			endTransaction <- i
			if err != nil || written == 0 {
				fileWatcher.Failed[i] = 1
				fileWatcher.FailedSum += 1
			} else {
				fileWatcher.Comleted[i] = 1
				fileWatcher.CompletedSum += 1
			}

		}(min, max, fileWatcher.Parts[j].Idx)

	}
	wg.Wait()

	for i := 0; i < len(fileWatcher.Parts); i++ {
		if fileWatcher.Comleted[i] == int64(0) {
			writeFileHelperToDir(fileWatcher, filewatcherFilename)
			return false
		}
	}

	fileWatcher.AllComplete = true
	// release wg semaphore
	writeFileHelperToDir(fileWatcher, filewatcherFilename)
	return fileWatcher.AllComplete
}

func remove(slice []Parts, s int64) []Parts {
	return append(slice[:s], slice[s+1:]...)
}

func writeFileHelperToDir(data Helper, path string) {
	file, _ := json.MarshalIndent(data, "", " ")
	_ = ioutil.WriteFile(path, file, 0644)
}

func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
