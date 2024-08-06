package common

import (
	"bytes"
	"text/template"
)

func ParseTemplate(input string, data map[string]interface{}) (string, error) {
	return ParseTemplates(input, data, nil, nil)
}

func ParseTemplates(input string, data map[string]any, funcMap template.FuncMap, extraInputs map[string]string) (string, error) {
	tmpl, err := template.New("tmpl").Funcs(funcMap).Parse(input)
	if err != nil {
		return "", err
	}
	for name, extraInput := range extraInputs {
		tmpl, err = tmpl.New(name).Parse(extraInput)
		if err != nil {
			return "", err
		}
	}
	buf := new(bytes.Buffer)
	if err := tmpl.ExecuteTemplate(buf, "tmpl", data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
