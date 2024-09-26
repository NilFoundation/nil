package api

//go:generate go run github.com/matryer/moq -out task_request_handler_generated_mock.go -rm -stub -with-resets . TaskRequestHandler
//go:generate go run github.com/matryer/moq -out task_handler_generated_mock.go -rm -stub -with-resets . TaskHandler
