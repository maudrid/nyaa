package main

import (
	"bufio"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v3"
)

type TConfig struct {
	EndPoint string   `yaml:"endPoint"`
	BaseUrl  string   `yaml:"baseUrl"`
	Filters  []string `yaml:"filters"`
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
		return nil, errors.New("Error: Invalid number of parameters")
	}
	rv := make(map[string]string)
	for ix, x := range args {
		if x[:1] == "-" {
			rv[x] = args[ix+1]
		}
	}
	return rv, nil
}

func getLinks(filter string) (items []TItem, err error) {
	resp, err := http.Get(filter)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	xmlResponse := TXmlResponseType{}
	err = xml.Unmarshal(body, &xmlResponse)
	if err != nil {
		return nil, err
	}
	return xmlResponse.Channel.Items, nil
}

func getRoot(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("got / request\n")

	io.WriteString(w, "ROOT, HTTP!\n")
}

func getLinksHttp(w http.ResponseWriter, r *http.Request) {
	//fmt.Printf("%v", configuration.Filters)
	var links []TItem
	for i, filter := range configuration.Filters {
		fmt.Println("Trying:", i, configuration.BaseUrl+filter)
		subLinks, err := getLinks(configuration.BaseUrl + filter)
		if err == nil {
			links = append(links, subLinks...)
		} else {
			fmt.Println("Warning", err)
		}
	}
	//fmt.Printf("%v", links)

	var response TXmlResponseType
	response.Version = "2.0"
	response.Channel.Items = links
	result, err := xml.Marshal(response)
	if err != nil {
		println("error:", err)
	} else {
		println("Writing response...")
	}
	w.Header().Set("Content-Type", "application/xml")
	io.WriteString(w, string(result))
}

var configuration TConfig

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
	http.HandleFunc("/links", getLinksHttp)
	fmt.Println("Starting webserver on", configuration.EndPoint)
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
				fmt.Println("modified file:", event.Name)
				configFromYamlFile(args) //This is bad maybe? To access memory (global variable) that other thread may be using.
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			fmt.Println("error:", err)
		}
	}
}
