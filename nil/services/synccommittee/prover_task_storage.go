package synccommittee

// Interface for updating task entries
type ProverTaskStorage interface {
	AddTaskEntry(entry ProverTaskEntry) error
	RemoveTaskEntry(id ProverTaskId) error
	RequestTaskToExecute(executor ProverId) (ProverTask, error)
	ProcessTaskResult(res ProverTaskResult) error
}
