package common

import (
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

type MockValue struct {
	value string
}

func (m *MockValue) Set(val string) error {
	m.value = val
	return nil
}

func (m *MockValue) String() string {
	return m.value
}

func (m *MockValue) Type() string {
	return "mock"
}

func (m *MockValue) UnmarshalText(input []byte) error {
	return m.Set(string(input))
}

func (m MockValue) MarshalText() ([]byte, error) {
	return []byte(m.String()), nil
}

func TestValueSlice_Set(t *testing.T) {
	t.Parallel()

	var vs PValueSlice[*MockValue, MockValue]
	err := vs.Set("val1,val2,val3")
	require.NoError(t, err)
	require.Len(t, vs, 3)
	require.Equal(t, "val1", vs[0].String())
	require.Equal(t, "val2", vs[1].String())
	require.Equal(t, "val3", vs[2].String())
}

func TestValueSlice_String(t *testing.T) {
	t.Parallel()

	vs := PValueSlice[*MockValue, MockValue]{
		MockValue{value: "val1"},
		MockValue{value: "val2"},
		MockValue{value: "val3"},
	}
	str := vs.String()
	require.Equal(t, "val1,val2,val3", str)
}

func TestValueSlice_Type(t *testing.T) {
	t.Parallel()

	var vs PValueSlice[*MockValue, MockValue]
	require.Equal(t, "PValueSlice[mock]", vs.Type())
}

type TestConfig struct {
	Slice PValueSlice[*MockValue, MockValue] `yaml:"slice"`
}

func TestValueSlice_MarshalYAML(t *testing.T) {
	t.Parallel()

	cfg := TestConfig{
		Slice: PValueSlice[*MockValue, MockValue]{
			MockValue{value: "val1"},
			MockValue{value: "val2"},
			MockValue{value: "val3"},
		},
	}
	data, err := yaml.Marshal(&cfg)
	require.NoError(t, err)
	require.Equal(t, "slice:\n    - val1\n    - val2\n    - val3\n", string(data))
}

func TestValueSlice_UnmarshalYAML(t *testing.T) {
	t.Parallel()

	test := func(data []byte) {
		var cfg TestConfig
		err := yaml.Unmarshal(data, &cfg)
		require.NoError(t, err)
		require.Len(t, cfg.Slice, 3)
		require.Equal(t, "val1", cfg.Slice[0].String())
		require.Equal(t, "val2", cfg.Slice[1].String())
		require.Equal(t, "val3", cfg.Slice[2].String())
	}
	t.Run("One line", func(t *testing.T) {
		t.Parallel()

		test([]byte("slice: [val1, val2, val3]"))
	})
	t.Run("Multiple lines", func(t *testing.T) {
		t.Parallel()

		test([]byte("slice:\n - val1\n - val2\n - val3\n"))
	})
}

func TestValueSlice_PFlag(t *testing.T) {
	t.Parallel()

	var vs PValueSlice[*MockValue, MockValue]
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	fs.Var(&vs, "values", "comma-separated list of values")

	err := fs.Parse([]string{"--values=val1,val2,val3"})
	require.NoError(t, err)
	require.Len(t, vs, 3)
	require.Equal(t, "val1", vs[0].String())
	require.Equal(t, "val2", vs[1].String())
	require.Equal(t, "val3", vs[2].String())
}

func TestValueSlice_PFlagDefault(t *testing.T) {
	t.Parallel()

	var vs PValueSlice[*MockValue, MockValue]
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	fs.Var(&vs, "values", "comma-separated list of values")

	err := fs.Parse([]string{})
	require.NoError(t, err)
	require.Empty(t, vs)
}
