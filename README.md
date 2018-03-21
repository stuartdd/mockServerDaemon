# mockServerDaemon
Register a port and test to see if was used before

This is a **small** server that keeps track of multiple port numbers during a large parallel gradle build.

The issue if that MockServer which we use to mock downstream systems hoggs the port. This is not an issue for non parallel tests.

This tool does not allocate ports, it is used to cause the build to fail if the same port is used twice.

The tools is started at the start of the build by the code that starts the MockServer and times out after n seconds of inactivity.

_This is the first time I have used git repository so please comment on the repo and the code if you feel the need :-)_

