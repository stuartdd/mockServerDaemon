// deamon.go
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type pmAction int

const (
	PM_READ_UPDATE pmAction = iota
	PM_CLEAR
	PM_LIST
)

const RESP_TEST = "{\"test\":\"%s\", \"state\":\"%s\", \"note\":\"%s\", \"free\":\"%s\"}"
const RESP_ACTION = "{\"action\":\"%s\", \"state\":\"%s\", \"note\":\"%s\", \"timeout\":%d, \"inuse\":%d}"
const RESP_LIST = "{\"action\":\"list\", \"state\":\"ok\", \"ports\":\"%s\", \"inuse\":%d}"
const CONFIG_DATA = "CFG{\"port\":%d, \"minPort\":%d, \"maxPort\":%d, \"minTimeout\":%d, \"maxTimeout\":%d, \"timeout\":%d, \"logfilename\":\"%s\"}"
const PORT_MIN = 8000
const PORT_MAX = 8999
const TIMEOUT_MIN = 5
const TIMEOUT_MAX = 300
const TIMEOUT_DEFAULT = 20
const FLAG = "F"

var portmap map[string]string
var stopAtSeconds int64 = 0
var lastUsedTimeoutValue int64 = 0
var thisServerPort = ""
var serverName = ""
var configData *Config
var criticalMutex sync.Mutex
var logFile *os.File

/*
Date read from the JSON configuration file. Note any undefined values are defaulted
to constants defined in  this program
*/
type Config struct {
	Debug       bool
	Port        int
	MinPort     int
	MaxPort     int
	MinTimeout  int
	MaxTimeout  int
	Timeout     int64
	LogFileName string
	ConfigName  string
}

/*
To string the configuration data. Used to record it in the logs
*/
func (p *Config) toString() string {
	return fmt.Sprintf(CONFIG_DATA, p.Port, p.MinPort, p.MaxPort, p.MinTimeout, p.MaxTimeout, p.Timeout, p.LogFileName)
}

func main() {
	/*
		Read the configuration file. If no name is given use the default name.
	*/
	var configFileName string

	if len(os.Args) > 1 {
		configFileName = os.Args[1]
		if !strings.HasSuffix(strings.ToLower(configFileName), ".json") {
			configFileName = configFileName + ".json"
		}
	} else {
		configFileName = "mockServerDaemon.json"
	}

	config, err := Load(configFileName)

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	if config == nil {
		log.Println("Config file is null!")
		os.Exit(1)
	}

	RunWithConfig(config)
}

func RunWithConfig(config *Config) {

	configData = config

	/*
		Open the logs. Log name is in the congig data. If not defined default to sysout
	*/
	createLog()
	defer closeLog()

	/*
		Get the name of this executable. It is returned in the http header.
	*/
	serverName, _ = os.Executable()

	/*
	   Say hello.
	*/
	log.Printf("Server will start on port %d\n", config.Port)
	log.Printf("To stop the server http://localhost:%d/stop\n", config.Port)

	if configData.Debug {
		log.Println(configData.toString())
	}

	/*
	   Clear and init the port map
	*/
	protectedPortMapCode(&criticalMutex, "", PM_CLEAR)

	/*
	   Set the time out for the server. If no activity then the server closes
	*/
	setTimeoutSeconds(config.Timeout)
	go doTimeout()

	/*
	   Map http requests to functions
	*/
	http.HandleFunc("/test/", testHandler)
	http.HandleFunc("/reset", resetHandler)
	http.HandleFunc("/stop", stopHandler)
	http.HandleFunc("/status", statusHandler)
	http.HandleFunc("/ping", pingHandler)
	http.HandleFunc("/list", listHandler)
	http.HandleFunc("/timeout/", timeoutHandler)
	/*
	   Start the server.
	*/
	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(config.Port), nil))
}

/*
	This code protects the port map from concurrent processes. All access must go through here!
	The Mutex prevents multiple threads running the code.

	The defer ensures that the lock is always lifted when the method exits.

	This will be updated with go 1.9 to use a concurrent map.
*/
func protectedPortMapCode(m *sync.Mutex, portToTest string, action pmAction) (string, bool) {
	m.Lock()
	defer m.Unlock()

	switch action {
	case PM_LIST:
		var buffer bytes.Buffer
		for k, _ := range portmap {
			buffer.WriteString(k)
			buffer.WriteString(",")
		}
		return strings.Trim(buffer.String(), ","), false
	case PM_READ_UPDATE:
		flag := portmap[portToTest]
		if flag == "" {
			portmap[portToTest] = FLAG
			return "", true
		}
		for j := configData.MinPort; j <= configData.MaxPort; j++ {
			testport := strconv.Itoa(j)
			if portmap[testport] == "" {
				return testport, false
			}
		}
		return "NONE", false
	case PM_CLEAR:
		portmap = make(map[string]string)
		portmap[strconv.Itoa(configData.Port)] = strconv.Itoa(configData.Port)
	}
	return "", true
}

/************************************************
Start of handlers section
*************************************************/
func testHandler(w http.ResponseWriter, r *http.Request) {
	resetTimeout()
	setHeaders(w)
	pathElements := strings.Split(r.URL.Path, "/")
	testPort := pathElements[2]
	valid, note, _ := portInvalid(testPort)
	if valid != "" {
		fmt.Fprintf(w, respondTest(testPort, valid, note, "0000", r.URL.Path))
	} else {
		unUsedPort, isUnUsed := protectedPortMapCode(&criticalMutex, testPort, PM_READ_UPDATE)
		if isUnUsed {
			fmt.Fprintf(w, respondTest(testPort, "pass", "Can be used", testPort, r.URL.Path))
		} else {
			fmt.Fprintf(w, respondTest(testPort, "fail", "Port is in use", unUsedPort, r.URL.Path))
		}
	}
}

func stopHandler(w http.ResponseWriter, r *http.Request) {
	go stopServer(false)
	setHeaders(w)
	fmt.Fprintf(w, respondAction("STOP", "OK", "", r.URL.Path))
}

func resetHandler(w http.ResponseWriter, r *http.Request) {
	protectedPortMapCode(&criticalMutex, "", PM_CLEAR)
	resetTimeout()
	setHeaders(w)
	fmt.Fprintf(w, respondAction("RESET", "OK", "", r.URL.Path))
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
	resetTimeout()
	setHeaders(w)
	fmt.Fprintf(w, respondAction("STATUS", "OK", "", r.URL.Path))
}

func listHandler(w http.ResponseWriter, r *http.Request) {
	resetTimeout()
	setHeaders(w)
	fmt.Fprintf(w, respondList(r.URL.Path))
}

func pingHandler(w http.ResponseWriter, r *http.Request) {
	setHeaders(w)
	fmt.Fprintf(w, respondAction("PING", "OK", "", r.URL.Path))
}

func timeoutHandler(w http.ResponseWriter, r *http.Request) {
	resetTimeout()
	pathElements := strings.Split(r.URL.Path, "/")
	valid, note, timeout := timeoutInvalid(pathElements[2])
	setHeaders(w)
	if valid != "" {
		fmt.Fprintf(w, respondAction("TIMEOUT", valid, note, r.URL.Path))
	} else {
		setTimeoutSeconds(timeout)
		fmt.Fprintf(w, respondAction("TIMEOUT", "valid", "", r.URL.Path))
	}
}

/************************************************
End of handlers section

Start of utility functions
*************************************************/
func createLog() {
	if configData.LogFileName != "" {
		f, err := os.OpenFile(configData.LogFileName, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			fmt.Printf("Log file '%s' could NOT be opened\nError:%s", configData.LogFileName, err.Error())
			return
		}
		logFile = f
		log.SetOutput(logFile)
	}
}

func closeLog() {
	if logFile != nil {
		logFile.Close()
	}
}

func respondAction(name string, ok string, note string, path string) string {
	s := fmt.Sprintf(RESP_ACTION, name, ok, note, getSecondsRemaining(), len(portmap))
	log.Printf("REQ{\"url\":\"%s\"} RES%s", path, s)
	return s
}

func respondTest(testport string, state string, note string, text string, path string) string {
	s := fmt.Sprintf(RESP_TEST, testport, state, note, text)
	log.Printf("REQ{\"url\":\"%s\"} RES%s", path, s)
	return s
}

func respondList(path string) string {
	l, _ := protectedPortMapCode(&criticalMutex, "", PM_LIST)
	s := fmt.Sprintf(RESP_LIST, l, getSecondsRemaining())
	log.Printf("REQ{\"url\":\"%s\"} RES%s", path, s)
	return s
}

func setHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Server", serverName)
}

func stopServer(immediate bool) {
	if !immediate {
		time.Sleep(time.Millisecond * 500)
	}
	closeLog()
	os.Exit(0)
}

func doTimeout() {
	for true {
		time.Sleep(time.Millisecond * 1000)
		if getSecondsRemaining() < 0 {
			stopServer(true)
		}
	}
}

func getSecondsRemaining() int64 {
	return stopAtSeconds - time.Now().Unix()
}

func resetTimeout() {
	setTimeoutSeconds(lastUsedTimeoutValue)
}

func setTimeoutSeconds(t int64) {
	lastUsedTimeoutValue = t
	stopAtSeconds = time.Now().Unix() + t
}

func timeoutInvalid(p string) (string, string, int64) {
	value, err := strconv.Atoi(p)
	if err != nil {
		return "format", "invalid integer format", 0
	}
	if (value < configData.MinTimeout) || (value > configData.MaxTimeout) {
		return "range", "< " + strconv.Itoa(configData.MinTimeout) + " or > " + strconv.Itoa(configData.MaxTimeout), 0
	}
	return "", "", int64(value)
}

func portInvalid(p string) (string, string, int) {
	port, err := strconv.Atoi(p)
	if err != nil {
		return "format", "invalid integer format", port
	}
	if (port < configData.MinPort) || (port > configData.MaxPort) {
		return "range", "< " + strconv.Itoa(configData.MinPort) + " or > " + strconv.Itoa(configData.MaxPort), port
	}
	return "", "", port
}

func Load(fileName string) (*Config, error) {
	config := Config{
		Timeout:    TIMEOUT_DEFAULT,
		MaxPort:    PORT_MAX,
		MinPort:    PORT_MIN,
		Port:       (PORT_MIN - 1),
		MaxTimeout: TIMEOUT_MAX,
		MinTimeout: TIMEOUT_MIN}

	b, err := LoadFile(fileName)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(b, &config)
	if err != nil {
		return nil, err
	}
	config.ConfigName = fileName
	return &config, nil
}

func LoadFile(fileName string) ([]byte, error) {
	raw, err := ioutil.ReadFile(fileName)
	if err != nil {
		return nil, fmt.Errorf("Failed to Load config data [%s]", fileName)
	}
	return raw, nil
}
