package commands

import (
	"strings"
	"unicode"
)

const separator = ','

type TaskFieldsFlag struct {
	FieldsToInclude *[]TaskField
}

func (f TaskFieldsFlag) String() string {
	return strings.Join(*f.FieldsToInclude, string(separator))
}

func (f TaskFieldsFlag) Set(str string) error {
	values := strings.FieldsFunc(str, func(r rune) bool {
		return r == separator || unicode.IsSpace(r)
	})
	values = removeDuplicates(values)

	if len(values) == 0 {
		*f.FieldsToInclude = DefaultTaskFields()
		return nil
	}

	if len(values) == 1 && strings.ToLower(values[0]) == "all" {
		*f.FieldsToInclude = AllTaskFields()
		return nil
	}

	*f.FieldsToInclude = values
	return nil
}

func removeDuplicates(s []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, v := range s {
		if !seen[v] {
			seen[v] = true
			result = append(result, v)
		}
	}
	return result
}

func (TaskFieldsFlag) Type() string {
	return "TaskFields"
}
