# mockServerDaemon
Register a port and test to see if was used before

This is a **micro** server that keeps track of multiple port numbers during a large parallel gradle build.

It is not intended to be used 'out of the box' but may serve as an example of how to develop a micro server.

The issue for me was that MockServer which we use to mock downstream systems hoggs the port. This only an issue parallel testing. If two tests run simultainously the request the same port. This causes MockServer to fail with a bind error.

This tool does not allocate ports, it is used to cause the build to fail if the same port is used twice.

The tools is started as a daemon service at the start of the build by the code that allocates ports to the MockServer and times out after n seconds of inactivity.

_This is the first time I have used git repository so please comment on the repo and the code if you feel the need :-)_

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
## Run
mockServerDaemon configFileName
