package api

//go:generate bash ../scripts/generate_mock.sh TaskRequestHandler
//go:generate bash ../scripts/generate_mock.sh TaskHandler
//go:generate bash ../scripts/generate_mock.sh TaskStateChangeHandler

//go:generate stringer -type=TaskDebugOrder -trimprefix=TaskDebugOrder
