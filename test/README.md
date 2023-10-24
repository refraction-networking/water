
# `/test` - Integration Testing

Testing for Water WebAssembly failure mode handling.

The client library needs to handle errors and unexpected behavior gracefully and give reasonable &
informative information to the caller. Beyond this, even under unexpected confitions, the WATER
library must not do things like:
* leak memory
* leak goroutines
* panic - crashing the caller
* hang - deadlocking the caller

## Outer Failure Mode testing
Censors do a lot of weird things to network connections that can cause unusual network behavior. The
internet itself is also just an unpredictable environment. Our client needs to hande the following
network related problems and provide the caller with informative errors. 

* Dialer
	- dial error Unreachable
	- dial error Refused
    - dial error Timeout
	- error from network-side connection
	- error from caller-side connection
	- caller-side cancelation
	- network-side timeout (remote dissapears)
    - network side connection reset

* Listener - probably not required for client to handle, but maybe if running WATER on the server side.
	- connection not accepted by caller
    - process hits file descriptor limit


## Inner Failure Mode testing

On the other side of this coin we have the potential failure modes in the WebAssembly binaries
that run inside the WATER runtime. In these cases as well, we need to ensure that the water client
library retains control over the runtime and returns reasonable, and descriptive errors to the
caller for further handling.

* clean tunnel close
* error tunnel close
* encoder or decoder thread crashes
* handshake failure
* encode / decode error
* other wasm panic
* thread stuck on blocking read / write

## Future

**Water Binary Tester** - run a water binary through a series of tests to check for potential
improper handling of external failure modes.