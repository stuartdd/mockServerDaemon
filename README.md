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

## Config file
```golang 
Debug       bool    // default true: logs lots of things
Port        int     // default is MinPort - 1
MinPort     int     // default 8000: any port less than this is invalid
MaxPort     int     // default 8999: any port more than this is invalid
MinTimeout  int     // default 5:    any timeout less than this is invalid
MaxTimeout  int     // default 300:  any timeout more than this is invalid
Timeout     int64   // default 15:   time out at launch
LogFileName string  // "" the name of a log file. If undefined logs to console
``` 
## Test
In src/github.com/stuartdd/mockServerDaemon

```go test```

## Run Linux
```./mockServerDaemon configFileName```

## Run Windows
```mockServerDaemon.exe configFileName```

## Config file
The configFileName is optional.
If configFileName is not given the server will look for 'mockServerConfig.json' in the current path.
If configFileName is given without a suffix of '.json' the suffix will be added.

## API is ReST like.
Note ALL server requests are http GET methods.

### Stop the server
```/stop```

The response will be:

```{"action":"STOP", "state":"OK", "note":"", "timeout":197, "inuse":1}```

The server will terminate about a second later!

### Test a port
```/test/<port>```

Note the timeout will be reset by this Action.
Example ```http://server:7999/test/8500```
  
If the port is already in use the response will be:
  
```{"test":"<port>", "state":"fail", "note":"Port is in use", "free":"<freeport>"}```

Note **freeport** is a port that can be used and is currently free.

If it is NOT already in use the response will be:

```{"test":"<port>", "state":"pass", "note":"Can be used", "free":"<port>"}```

If the port is outside the range defined in the configuration file the following response is returned:

```{"test":"<port>", "state":"range", "note":"< 8000 or > 9000", "free":"0000"}```

If the port is an invalid integer the following response is returned:

```{"test":"<port>", "state":"format", "note":"invalid integer format", "free":"0000"}```

### Get server status
```/status```

Note the **timeout** will be reset by this Action.

Example ```http://server:7999/status```

Response:

```{"action":"STATUS", "state":"OK", "note":"", "timeout":300, "inuse":2}```

Note that **timeout** is reset to the value defined in the configuration file.
The **inuse** value is the current number of ports that have been tested plus the port the server is using.

### Reset the server
```/reset```

Note the **timeout** will be reset by this Action.

Example ```http://server:7999/reset```

Response:

```{"action":"RESET", "state":"OK", "note":"", "timeout":300, "inuse":1}```

Note that **timeout** is reset to the value defined in the configuration file.
The **inuse** value will always be 1 as the port the server is using is included in the list.

### Ping the server
```/ping```

Note the timeout will **NOT** be reset by this Action.

Example ```http://server:7999/ping```

Response:

```{"action":"PING", "state":"OK", "note":"", "timeout":150, "inuse":6}```

Note that **timeout** is the remaining time the server will run if no other activity is detected.
The **inuse** value is the current number of ports that have been tested plus the port the server is using.

### Set the server timeout
```/timeout/<seconds>```

Note the timeout is in seconds.

Example ```http://server:7999/timeout/300```

Response if the timeout value is valid and accepted:

```{"action":"TIMEOUT", "state":"valid", "note":"", "timeout":<seconds>, "inuse":2}```

Note that **seconds** should be the time set but an edge case can result in the value being decremented.

Response if the timeout value is outside the range set in the configuration file:

```{"action":"TIMEOUT", "state":"range", "note":"< 5 or > 300", "timeout":<seconds>, "inuse":2}```

Note that **seconds** will be the original timeout value.

Response if the timeout value is not a valid integer:

```{"action":"TIMEOUT", "state":"format", "note":"invalid integer format", "timeout":300, "inuse":2}```

Note that **seconds** will be the original timeout value.
