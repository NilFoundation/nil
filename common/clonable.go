package common

type Clonable interface {
	Clone() Clonable
}
