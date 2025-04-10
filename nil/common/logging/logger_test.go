package logging

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLogger(t *testing.T) {
	t.Parallel()

	SetupGlobalLogger("debug")

	log1Buf := new(bytes.Buffer)
	log2Buf := new(bytes.Buffer)
	log3Buf := new(bytes.Buffer)

	log1 := NewLoggerWithWriter("log1", log1Buf)
	log2 := NewLoggerWithWriter("log2", log2Buf)
	log3 := NewLoggerWithWriter("log3", log3Buf)

	msgIndex := 0
	emitLogs := func() {
		log1Buf.Reset()
		log2Buf.Reset()
		log3Buf.Reset()
		msgIndex++
		log1.Warn().Msgf("log1 message %d", msgIndex)
		log2.Warn().Msgf("log2 message %d", msgIndex)
		log3.Warn().Msgf("log3 message %d", msgIndex)
	}

	ApplyComponentsFilter("-all")
	emitLogs()
	require.Equal(t, 0, log1Buf.Len())
	require.Equal(t, 0, log2Buf.Len())
	require.Equal(t, 0, log3Buf.Len())

	ApplyComponentsFilter("all:-log1")
	emitLogs()
	require.Equal(t, 0, log1Buf.Len())
	require.Contains(t, log2Buf.String(), fmt.Sprintf("log2 message %d", msgIndex))
	require.Contains(t, log3Buf.String(), fmt.Sprintf("log3 message %d", msgIndex))

	ApplyComponentsFilter("log1:-all")
	emitLogs()
	require.Equal(t, 0, log1Buf.Len())
	require.Equal(t, 0, log2Buf.Len())
	require.Equal(t, 0, log3Buf.Len())

	ApplyComponentsFilter("-all:-log3:all")
	emitLogs()
	require.Contains(t, log1Buf.String(), fmt.Sprintf("log1 message %d", msgIndex))
	require.Contains(t, log2Buf.String(), fmt.Sprintf("log2 message %d", msgIndex))
	require.Contains(t, log3Buf.String(), fmt.Sprintf("log3 message %d", msgIndex))

	logBuf := new(bytes.Buffer)

	ApplyComponentsFilter("log4")
	log4 := NewLoggerWithWriter("log4", logBuf)
	log4.Warn().Msgf("log4 message %d", msgIndex)
	require.Contains(t, logBuf.String(), fmt.Sprintf("log4 message %d", msgIndex))

	logBuf.Reset()
	ApplyComponentsFilter("-log5")
	log5 := NewLoggerWithWriter("log5", logBuf)
	log5.Warn().Msgf("log5 message %d", msgIndex)
	require.Equal(t, 0, logBuf.Len())

	ApplyComponentsFilter("-al:-log:dsf::og9erkjthdk&*%^*#s--flk:jsd:lfk3")
	emitLogs()
	require.Contains(t, log1Buf.String(), fmt.Sprintf("log1 message %d", msgIndex))
	require.Contains(t, log2Buf.String(), fmt.Sprintf("log2 message %d", msgIndex))
	require.Contains(t, log3Buf.String(), fmt.Sprintf("log3 message %d", msgIndex))
}
