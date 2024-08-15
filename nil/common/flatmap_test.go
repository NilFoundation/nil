package common

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

type ReversedIntComparator struct{}

func (c ReversedIntComparator) Compare(a int, b int) int {
	if a > b {
		return -1
	}
	if a < b {
		return 1
	}
	return 0
}

type FlatMapWithReversedIntComparator = FlatMap[int, string, ReversedIntComparator]

func NewFlatMapWithReversedIntComparator(initialValues map[int]string) FlatMapWithReversedIntComparator {
	return NewFlatMap[int, string, ReversedIntComparator](initialValues)
}

func checkItems(t *testing.T, fm FlatMapWithReversedIntComparator, expected []string) {
	t.Helper()
	require.Len(t, fm.Items, len(expected))
	for i, pair := range fm.Items {
		require.Equal(t, expected[i], pair.Value)
	}
}

func TestFlatMap(t *testing.T) {
	t.Parallel()

	fm := NewFlatMapWithReversedIntComparator(map[int]string{
		1: "one",
		2: "two",
		4: "four",
		5: "five",
	})

	// Initial items order
	checkItems(t, fm, []string{"five", "four", "two", "one"})

	// String representation
	require.Equal(t, "{5: five, 4: four, 2: two, 1: one}", fm.String())

	// JSON serialization
	j, err := json.Marshal(fm)
	require.NoError(t, err)
	require.Equal(t, `{"5":"five","4":"four","2":"two","1":"one"}`, string(j))

	// JSON deserialization
	fm2 := FlatMapWithReversedIntComparator{}
	err = json.Unmarshal(j, &fm2)
	require.NoError(t, err)
	require.Equal(t, fm, fm2)

	// Continue to use the correct comparator for the deserialized object
	fm2.Set(3, "three")
	checkItems(t, fm2, []string{"five", "four", "three", "two", "one"})

	// Get existing
	v, ok := fm.Get(2)
	require.True(t, ok)
	require.Equal(t, "two", v)

	// Get non-existing
	_, ok = fm.Get(6)
	require.False(t, ok)

	// Set in the middle
	fm.Set(3, "three")
	checkItems(t, fm, []string{"five", "four", "three", "two", "one"})

	// Set in the beginning
	fm.Set(6, "six")
	checkItems(t, fm, []string{"six", "five", "four", "three", "two", "one"})

	// Set in the end
	fm.Set(0, "zero")
	checkItems(t, fm, []string{"six", "five", "four", "three", "two", "one", "zero"})

	// Delete from the beginning
	fm.Delete(0)
	checkItems(t, fm, []string{"six", "five", "four", "three", "two", "one"})

	// Delete from the end
	fm.Delete(6)
	checkItems(t, fm, []string{"five", "four", "three", "two", "one"})

	// Delete from the middle
	fm.Delete(3)
	checkItems(t, fm, []string{"five", "four", "two", "one"})
}
