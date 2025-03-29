package srv

import "github.com/NilFoundation/nil/nil/common/logging"

func WorkerLogger(logger logging.Logger, worker Worker) logging.Logger {
	return logger.With().Str("worker", worker.Name()).Logger()
}
