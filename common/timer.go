package common

import (
	"time"
)

type Timer interface {
	Now() uint64
}

type RealTimerImpl struct{}

var _ Timer = new(RealTimerImpl)

func (t *RealTimerImpl) Now() uint64 {
	return uint64(time.Now().Unix())
}

type TestTimerImpl struct {
	NowTime uint64
}

func (t *TestTimerImpl) Now() uint64 {
	return t.NowTime
}

var realTimer = RealTimerImpl{}

func NewTimer() *RealTimerImpl {
	return &realTimer
}

func NewTestTimer(nowTime uint64) *TestTimerImpl {
	return &TestTimerImpl{NowTime: nowTime}
}
