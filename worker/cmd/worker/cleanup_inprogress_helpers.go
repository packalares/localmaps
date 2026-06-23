package main

import "os"

// osRemove is a thin shim around os.Remove so tests can stub it via
// cleanup_inprogress.go's `removeFile` var without pulling in a full
// filesystem abstraction.
var osRemove = os.Remove
