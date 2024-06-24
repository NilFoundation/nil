package common

import (
	"bytes"
	"text/template"
)

func ParseTemplate(input string, data map[string]interface{}) (string, error) {
	tmpl, err := template.New("tmpl").Parse(input)
	if err != nil {
		return "", err
	}
	buf := new(bytes.Buffer)
	if err := tmpl.Execute(buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
