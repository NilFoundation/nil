package api

//go:generate go run github.com/matryer/moq -out task_request_handler_generated_mock.go -rm -stub -with-resets . TaskRequestHandler
//go:generate bash -c "echo -e \"//go:build test\n$(cat task_request_handler_generated_mock.go)\" > task_request_handler_generated_mock.go"

//go:generate go run github.com/matryer/moq -out task_handler_generated_mock.go -rm -stub -with-resets . TaskHandler
//go:generate bash -c "echo -e \"//go:build test\n$(cat task_handler_generated_mock.go)\" > task_handler_generated_mock.go"
