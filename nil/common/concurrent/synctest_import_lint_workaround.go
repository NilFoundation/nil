//go:build test

package concurrent

// NOTE: If your IDE does not find this import, add goexperiment.synctest to the build tags.
import "testing/synctest"

// Since testing/synctest is an experimental package, linters go crazy trying to organize its import among others.
// Therefore, we put it in a separate file and use the functions from the package through.
var (
	synctestRun  = synctest.Run
	synctestWait = synctest.Wait
)
