// Package handler — small helpers shared by the router and handlers.
package handler

import "time"

// timeNow is overridable in tests.
var timeNow = time.Now
