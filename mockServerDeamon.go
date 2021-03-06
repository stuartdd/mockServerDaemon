package main

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	jsonconfig "github.com/stuartdd/tools_jsonconfig"
)

type pmAction int

const (
	pmReadUpdateConst pmAction = iota
	pmClearConst
	pmListConst
	pmFreeConst
)

const respTestConst = "{\"test\":\"%s\", \"state\":\"%s\", \"note\":\"%s\", \"free\":\"%s\"}"
const respActionConst = "{\"action\":\"%s\", \"state\":\"%s\", \"note\":\"%s\", \"timeout\":%d, \"inuse\":%d}"
const respListConst = "{\"action\":\"list\", \"state\":\"ok\", \"ports\":\"%s\", \"inuse\":%d}"
const defaultConfigConst = "CFG{\"port\":%d, \"minPort\":%d, \"maxPort\":%d, \"minTimeout\":%d, \"maxTimeout\":%d, \"timeout\":%d, \"logfilename\":\"%s\"}"
const portMin = 8000
const portMax = 8999
const timeOutMinConst = 5
const timeOutMaxConst = 300
const tomeoutDefaultConst = 20
const flagConst = "F"

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
	return fmt.Sprintf(defaultConfigConst, p.Port, p.MinPort, p.MaxPort, p.MinTimeout, p.MaxTimeout, p.Timeout, p.LogFileName)
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

	/*
		Create a config object with the default values
	*/
	configData := Config{
		Timeout:    tomeoutDefaultConst,
		MaxPort:    portMax,
		MinPort:    portMin,
		Port:       (portMin - 1),
		MaxTimeout: timeOutMaxConst,
		MinTimeout: timeOutMinConst}

	/*
		load the config object
	*/
	err := jsonconfig.LoadJson(configFileName, &configData)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	RunWithConfig(&configData)
}

// RunWithConfig - runs with a specific configuration object
//	Param - config a ref to the config object
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
	log.Printf("Server will start on port %d\n", configData.Port)
	log.Printf("To stop the server http://localhost:%d/stop\n", configData.Port)
	log.Print("Actions:\nstop - Stop the server\nstatus - Return server status\ntest/{port} - Test a port\nlist - Show used port list\nreset - Clear all ports")
	if configData.Debug {
		log.Println(configData.toString())
	}

	/*
	   Clear and init the port map
	*/
	protectedPortMapCode(&criticalMutex, "", pmClearConst)

	/*
	   Set the time out for the server. If no activity then the server closes
	*/
	setTimeoutSeconds(configData.Timeout)
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
	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(configData.Port), nil))
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
	case pmFreeConst:
		for j := configData.MinPort; j <= configData.MaxPort; j++ {
			testport := strconv.Itoa(j)
			if portmap[testport] == "" {
				return testport, true
			}
		}
	case pmListConst:
		var buffer bytes.Buffer
		for k := range portmap {
			buffer.WriteString(k)
			buffer.WriteString(",")
		}
		return strings.Trim(buffer.String(), ","), false
	case pmReadUpdateConst:
		flag := portmap[portToTest]
		if flag == "" {
			portmap[portToTest] = flagConst
			return "", true
		}
		for j := configData.MinPort; j <= configData.MaxPort; j++ {
			testport := strconv.Itoa(j)
			if portmap[testport] == "" {
				return testport, false
			}
		}
		return "NONE", false
	case pmClearConst:
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
		free, _ := protectedPortMapCode(&criticalMutex, "", pmFreeConst)
		fmt.Fprintf(w, respondTest(testPort, valid, note, free, r.URL.Path))
	} else {
		unUsedPort, isUnUsed := protectedPortMapCode(&criticalMutex, testPort, pmReadUpdateConst)
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
	protectedPortMapCode(&criticalMutex, "", pmClearConst)
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
	s := fmt.Sprintf(respActionConst, name, ok, note, getSecondsRemaining(), len(portmap))
	log.Printf("REQ{\"url\":\"%s\"} RES%s", path, s)
	return s
}

func respondTest(testport string, state string, note string, text string, path string) string {
	s := fmt.Sprintf(respTestConst, testport, state, note, text)
	log.Printf("REQ{\"url\":\"%s\"} RES%s", path, s)
	return s
}

func respondList(path string) string {
	l, _ := protectedPortMapCode(&criticalMutex, "", pmListConst)
	s := fmt.Sprintf(respListConst, l, len(portmap))
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
