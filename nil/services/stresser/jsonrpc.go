package stresser

type StresserAPI interface {
	New(name, config string) error
	Remove(name string) error
	ListStressers() ([]string, error)
	GetStatus(name string) (string, error)
	Stop(name string) error
	Resume(name string) error
}

// TODO: implement
