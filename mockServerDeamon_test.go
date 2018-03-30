// deamon.go
package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

type ActionResp struct {
	Action  string
	State   string
	Note    string
	Timeout int
}

type TestResp struct {
	Test  string
	State string
	Note  string
	Free  string
}

func TestMain(m *testing.M) {

	go RunWithConfig(&Config{
		Timeout:    300,
		MaxPort:    9000,
		MinPort:    8000,
		Port:       7999,
		MinTimeout: 5,
		MaxTimeout: 300})

	os.Exit(m.Run())
}

func TestPorts(t *testing.T) {
	resp := get(t, "test/8500")
	assertContains(t, resp, "{\"test\":\"8500\", \"state\":\"pass\", \"note\":\"Can be used\", \"free\":\"8500\"}")
	testResp := getTestPort(t, "test/8500")
	assertEquals(t, testResp.State, "fail")

	for i := 0; i < 5; i++ {
		testResp = getTestPort(t, "test/"+testResp.Free)
		assertEquals(t, testResp.State, "pass")
		testResp = getTestPort(t, "test/"+testResp.Free)
		assertEquals(t, testResp.State, "fail")
	}

	testResp = getTestPort(t, "test/8500")
	assertEquals(t, testResp.State, "fail")
	testResp = getTestPort(t, "test/"+testResp.Free)
	assertEquals(t, testResp.State, "pass")

	getAction(t, "reset")
	testResp = getTestPort(t, "test/8500")
	assertEquals(t, testResp.State, "pass")

}

func TestStatus(t *testing.T) {
	resp := get(t, "status")
	assertContains(t, resp, "{\"action\":\"STATUS\", \"state\":\"OK\", \"note\":\"\", \"timeout\":")
	resp = get(t, "reset")
	assertContains(t, resp, "{\"action\":\"RESET\", \"state\":\"OK\", \"note\":\"\", \"timeout\":")
	resp = get(t, "ping")
	assertContains(t, resp, "{\"action\":\"PING\", \"state\":\"OK\", \"note\":\"\", \"timeout\":")

	resp = get(t, "timeout/1")
	assertContains(t, resp, "{\"action\":\"TIMEOUT\", \"state\":\"range\", \"note\":")
	resp = get(t, "timeout/999")
	assertContains(t, resp, "{\"action\":\"TIMEOUT\", \"state\":\"range\", \"note\":")
	resp = get(t, "timeout/200")
	assertContains(t, resp, "{\"action\":\"TIMEOUT\", \"state\":\"OK\", \"note\":\"\", \"timeout\":")

	resp = get(t, "test/80")
	assertContains(t, resp, "{\"test\":\"80\", \"state\":\"invalid\", \"note\":")
	resp = get(t, "test/999999")
	assertContains(t, resp, "{\"test\":\"999999\", \"state\":\"invalid\", \"note\":")

	actionResp := getAction(t, "timeout/200")

	actionResp = getAction(t, "ping")
	if actionResp.Timeout < 199 {
		t.Logf("Time out should not decrement that quick. Expected < 199 returned %d", actionResp.Timeout)
		t.FailNow()
	}

	lastPing := actionResp.Timeout

	time.Sleep(time.Second * 3)

	actionResp2 := getAction(t, "ping")
	lastPing2 := actionResp2.Timeout

	if lastPing2 >= (lastPing - 1) {
		t.Logf("Time out should decrement. Expected < %d returned %d", (lastPing - 1), lastPing2)
		t.FailNow()
	}
}

func get(t *testing.T, action string) string {
	resp, err := http.Get("http://localhost:7999/" + action)
	if err != nil {
		t.Logf("Please start the server: Could send get request '%s', Error:%s", action, err.Error())
		t.FailNow()
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Log("Could not read resp body")
		t.FailNow()
	}
	return string(body)
}

func getTestPort(t *testing.T, action string) *TestResp {
	resp := get(t, action)
	testResp := TestResp{}
	err := json.Unmarshal([]byte(resp), &testResp)
	if err != nil {
		t.Errorf("String [%s] could not be marshaled to an TestResp", resp)
		t.FailNow()
	}
	return &testResp
}

func getAction(t *testing.T, action string) *ActionResp {
	resp := get(t, action)
	actionResp := ActionResp{}
	err := json.Unmarshal([]byte(resp), &actionResp)
	if err != nil {
		t.Errorf("String [%s] could not be marshaled to an ActionResp", resp)
		t.FailNow()
	}
	return &actionResp
}

func assertContains(t *testing.T, actual string, expected string) {
	if strings.Contains(actual, expected) {
		return
	}
	t.Errorf("String actual [%s] does not contain [%s]", actual, expected)
	t.FailNow()
}

func assertEquals(t *testing.T, actual string, expected string) {
	if actual == expected {
		return
	}
	t.Errorf("String actual [%s] does not equal expected [%s]", actual, expected)
	t.FailNow()
}
