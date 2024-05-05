package main

import (
	"bufio"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v3"
)

type TFeed struct {
	BaseUrl   string   `yaml:"baseUrl"`
	Filters   []string `yaml:"filters"`
	RateLimit int      `yaml:"rateLimit"`
}
type TConfig struct {
	EndPoint string  `yaml:"endPoint"`
	Feeds    []TFeed `yaml:"feeds"`
	CacheTTL int     `yaml:"cacheTTL"`
}

type TItem struct {
	Title string `xml:"title"`
	Link  string `xml:"link"`
}

type TXmlResponseType struct {
	XMLName xml.Name `xml:"rss"`
	Version string   `xml:"version,attr"`
	Channel struct {
		Items []TItem `xml:"item"`
	} `xml:"channel"`
}

type TItems struct {
	Items []TItem
	Error error
}

func panicOnErr(e error) {
	if e != nil {
		panic(e)
	}
}

func readStdIn() (config []byte, err error) {
	reader := bufio.NewReader(os.Stdin)
	buf := make([]byte, 0, 4*1024)
	result := ""
	for {
		n, err := reader.Read(buf[:cap(buf)])
		buf = buf[:n]
		if n == 0 {
			if err == nil {
				continue
			}

			if err == io.EOF {
				break
			}
			return nil, err
		}
		result = result + string(buf)
		if err != nil && err != io.EOF {
			return nil, err
		}
	}
	return []byte(result), nil
}

func argsToMap(args []string) (map[string]string, error) {
	if len(args)%2 != 0 {
		return nil, errors.New("error: invalid number of parameters")
	}
	rv := make(map[string]string)
	for ix, x := range args {
		if x[:1] == "-" {
			rv[x] = args[ix+1]
		}
	}
	return rv, nil
}

func getLinks(filter string) (result TItems) {
	fmt.Println("Trying:", filter)
	resp, err := http.Get(filter)
	if err != nil {
		result := TItems{}
		result.Items = nil
		result.Error = err
		return result
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		result := TItems{}
		result.Items = nil
		result.Error = err
		return result
	}
	xmlResponse := TXmlResponseType{}
	err = xml.Unmarshal(body, &xmlResponse)
	if err != nil {
		result := TItems{}
		result.Items = nil
		result.Error = err
		return result
	}
	result.Items = xmlResponse.Channel.Items
	result.Error = nil
	return result
}

func fillCache() {
	//fmt.Printf("%v", configuration.Filters)
	var links []TItem
	channel := make(chan TItems)

	for _, feed := range configuration.Feeds {
		for _, filter := range feed.Filters {
			go func(filter string) { channel <- getLinks(feed.BaseUrl + filter) }(filter)
			time.Sleep(time.Duration(feed.RateLimit) * time.Millisecond)
		}
		for range feed.Filters {
			result := <-channel
			if result.Error == nil {
				links = append(links, result.Items...)
			} else {
				fmt.Println("Warning", result.Error)
			}
		}
		//fmt.Printf("%v", links)
	}
	responseCache.Channel.Items = links
}

func getLinksHttp(w http.ResponseWriter, r *http.Request) {
	result, err := xml.Marshal(responseCache)
	if err != nil {
		println("error:", err)
	} else {
		print("Writing response... ")
	}
	w.Header().Set("Content-Type", "application/xml")
	io.WriteString(w, string(result))
	println("Done.")
}

var configuration TConfig
var responseCache TXmlResponseType
var confCounter int8

func main() {
	args, err := argsToMap(os.Args[1:])
	panicOnErr(err)
	fileInfo, err := os.Stdin.Stat()
	panicOnErr(err)
	var configData []byte
	if (fileInfo.Mode() & os.ModeNamedPipe) != 0 {
		configData, err = readStdIn()
		panicOnErr(err)
		err = yaml.Unmarshal(configData, &configuration)
		panicOnErr(err)
	} else {
		if args["-f"] == "" {
			fmt.Println("Usage: \"nyaa -f <config file>\" or \"cat <configuration> | nyaa\" ")
			os.Exit(0)
		}
		configFromYamlFile(args)
		watcher, err := fsnotify.NewWatcher()
		panicOnErr(err)
		defer watcher.Close()
		go refreshConfig(watcher, args)
		fmt.Println("Watching", args["-f"], "for changes.")
		err = watcher.Add(args["-f"])
		panicOnErr(err)
	}
	responseCache.Version = "2.0"
	confCounter = 0
	http.HandleFunc("/links", getLinksHttp)
	fmt.Println("Filling Cache...")
	fillCache()
	if configuration.CacheTTL == 0 {
		configuration.CacheTTL = 3600
	}
	fmt.Println("Starting cache ttl timer (seconds):", configuration.CacheTTL)
	cacheTicker := time.NewTicker(time.Duration(configuration.CacheTTL) * time.Second)
	go func() {
		for range cacheTicker.C {
			fmt.Println("Evicting cache...")
			fillCache()
			fmt.Println("Evicting cache done.")
		}
	}()
	fmt.Println("Starting web server on", configuration.EndPoint)
	fmt.Println("Ctrl-C to stop.")
	err = http.ListenAndServe(configuration.EndPoint, nil)
	panicOnErr(err)
}

func configFromYamlFile(args map[string]string) {
	fileBuffer, err := os.ReadFile(args["-f"])
	panicOnErr(err)
	err = yaml.Unmarshal(fileBuffer, &configuration)
	panicOnErr(err)
}

func refreshConfig(watcher *fsnotify.Watcher, args map[string]string) {
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Has(fsnotify.Write) {
				confCounter++
				if confCounter == 2 {
					confCounter = 0
					fmt.Println("modified file:", event.Name)
					configFromYamlFile(args) //This is bad maybe? To access memory (global variable) that other thread may be using.
					fillCache()
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			fmt.Println("error:", err)
		}
	}
}
