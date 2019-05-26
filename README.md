# mockServerDaemon
Register a port and test to see if was used before

This is a **micro** server that keeps track of multiple port numbers during a large parallel gradle build.

It is not intended to be used 'out of the box' but may serve as an example of how to develop a micro server.

The issue for me was that MockServer which we use to mock downstream systems hoggs the port. This only an issue for parallel testing. If two tests run simultainously they can request the same port. This causes MockServer to fail with a bind error.

This tool does not allocate ports, it is used to cause the build to fail if the same port is used twice. It also returns an unused port in the response.

The tools is started as a daemon service at the start of the build by the Java code that allocates ports to the MockServer and times out after n seconds of inactivity.

Added ```func RunWithConfig(config *Config)``` to allow server to be started from another program. This is currently how the tests start the server. It means that the tests do not need the server running and that the configuration can be controlled for the test run. 

Tests now use TestMain() to start the server before running the tests. The tests now run to end.

_This is the first time I have used git repository so please comment on the repo and the code if you feel the need :-)_

I did find it confusing to set up this project especially as I wanted it to reflect the standard structure of a go project in the repository. I wanted to be able to clone into a standard go directory structure and at the moment I cannot do that without a bit of, post clone, fidling of go environment variables. Any hints would be welcome. 

## Build - Dependencies
to fetch the dependency from github - Use the following in the GOTATH directory:

```
go get github.com/stuartdd/tools_jsonconfig
```

## Config file structure
```java
Debug       bool    // default true: logs lots of things
Port        int     // default is MinPort - 1
MinPort     int     // default 8000: any port less than this is invalid
MaxPort     int     // default 8999: any port more than this is invalid
MinTimeout  int     // default 5:    any timeout less than this is invalid
MaxTimeout  int     // default 300:  any timeout more than this is invalid
Timeout     int64   // default 15:   time out at launch
LogFileName string  // "" the name of a log file. If undefined logs to console
``` 
## Config file example
```json
{"debug":true, "logFileName","ms.log", "port":8080}
```
Note - That the names are NOT case sensitive. The JSON UnMarshaling just tries it's best to match the names.
If the names do not match the resultant value will remain unchanged (the default values shown above).

If debug is true the contents of the configuration data will be output to the console or log.

## Test
In src/github.com/stuartdd/mockServerDaemon

```
go test
```

## Run Linux
```
./mockServerDaemon configFileName
```

## Run Windows
```
mockServerDaemon.exe configFileName
```

## Config file
The configFileName is optional. If configFileName is not given the server will look for 'mockServerConfig.json' in the current path.
If configFileName is given without a suffix of '.json' the suffix will be added.

## API is ReST like.
Note ALL server requests are http GET methods.

### Stop the server
http://localhost:7999/stop

The response will be:
```json
{"action":"STOP", "state":"OK", "note":"", "timeout":197, "inuse":1}
```

The server will terminate about a second later!

### Test a port
http://localhost:7999/test/{port}

Note the timeout will be reset by this Action.
  
If the port is already in use the response will be:
  
```json
{"test":"{port}", "state":"fail", "note":"Port is in use", "free":"{freeport}"}
```
Note **{port}** is the port that was tested.

Note **{freeport}** is a port that can be used and is currently free.

If it is NOT already in use the response will be:

```json
{"test":"{port}", "state":"pass", "note":"Can be used", "free":"{freeport}"}
```

If the port is outside the range defined in the configuration file the following response is returned:

```json
{"test":"{port}", "state":"range", "note":"< 8000 or > 9000", "free":"{freeport}"}
```

If the port is an invalid integer the following response is returned:

```json
{"test":"{port}", "state":"format", "note":"invalid integer format", "free":"{freeport}"}
```

### Get server status
http://localhost:7999/status

Note the **timeout** will be reset by this Action.

Response:
```json
{"action":"STATUS", "state":"OK", "note":"", "timeout":300, "inuse":2}
```

Note that **timeout** is reset to the value defined in the configuration file.
The **inuse** value is the current number of ports that are in use plus the port the server is using.

### Reset the server
http://localhost:7999/reset

Note the **timeout** will be reset by this Action.

Response:
```json
{"action":"RESET", "state":"OK", "note":"", "timeout":300, "inuse":1}
```

Note that **timeout** is reset to the value defined in the configuration file.
The **inuse** value will always be 1 as the port the server is using is included in the list.

### Ping the server
http://localhost:7999/ping

Note the **timeout** will **NOT** be reset by this Action.

Response:

```json
{"action":"PING", "state":"OK", "note":"", "timeout":150, "inuse":6}
```

Note that **timeout** is the remaining time the server will run if no other activity is detected.
The **inuse** value is the current number of ports that are in use plus the port the server is using.

### Set the server timeout
http://localhost:7999/timeout/{seconds}


Note the timeout is given in seconds.

Response if the timeout value is valid and accepted:

```json
{"action":"TIMEOUT", "state":"valid", "note":"", "timeout":{seconds}, "inuse":2}
```

Note that **seconds** should be the time set but an edge case can result in the value being decremented.

Response if the timeout value is outside the range set in the configuration file:

```json
{"action":"TIMEOUT", "state":"range", "note":"< 5 or > 300", "timeout":{seconds}, "inuse":2}
```

Note that **seconds** will be the original timeout value.

Response if the timeout value is not a valid integer:

```json
{"action":"TIMEOUT", "state":"format", "note":"invalid integer format", "timeout":{seconds}, "inuse":2}
```

Note that **seconds** will be the original timeout value.
