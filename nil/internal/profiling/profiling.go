package profiling

import (
	"net/http"
	_ "net/http/pprof" //nolint:gosec
	"strconv"
)

const DefaultPort = 6060

func Start(port int) {
	go func() {
		_ = http.ListenAndServe("localhost:"+strconv.Itoa(port), nil) //nolint:gosec
	}()
}
