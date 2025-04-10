package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"regexp"
	"strings"

	"github.com/rs/zerolog/log"
)

type Component struct {
	Title       string               `json:"title"`
	Type        string               `json:"type"`
	Description string               `json:"description"`
	Properties  map[string]Component `json:"properties,omitempty"`
	Required    []string             `json:"required,omitempty"`
}

type Method struct {
	Name        string   `json:"name"`
	Tags        []TagRef `json:"tags"`
	Summary     string   `json:"summary"`
	Description string   `json:"description"`
	Params      []Param  `json:"params"`
	Errors      []any    `json:"errors"`
	Result      Result   `json:"result"`
	Examples    []any    `json:"examples"`
}

type TagRef struct {
	Ref string `json:"$ref"`
}

type Param struct {
	Name   string    `json:"name"`
	Schema SchemaRef `json:"schema"`
}

type SchemaRef struct {
	Ref string `json:"$ref"`
}

type Result struct {
	Name   string    `json:"name"`
	Schema SchemaRef `json:"schema"`
}

func main() {
	if err := generate_spec(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func generate_spec() error {
	componentsFromAPI := generateComponentsFromFile("nil/services/rpc/jsonrpc/doc.go")
	componentsFromTypes := generateComponentsFromFile("nil/services/rpc/jsonrpc/types.go")
	componentsFromRpcTypes := generateComponentsFromFile("nil/services/rpc/types/types.go")
	components := make(map[string]Component)

	maps.Insert(components, maps.All(componentsFromTypes))
	maps.Insert(components, maps.All(componentsFromAPI))
	maps.Insert(components, maps.All(componentsFromRpcTypes))

	methods := generateMethodsFromFile("nil/services/rpc/jsonrpc/eth_api.go")
	openrpcSpec := map[string]any{
		"openrpc": "1.2.4",
		"info": map[string]any{
			"title":       "JSON-RPC API",
			"version":     "1.0.0",
			"description": "This section lists the JSON-RPC methods exposed by the =nil; RPC node.",
		},
		"methods": methods,
		"components": map[string]any{
			"schemas": components,
			"tags": map[string]any{
				"Accounts": map[string]any{
					"name":        "Accounts",
					"description": "Methods for interacting with accounts.",
				},
				"Blocks": map[string]any{
					"name":        "Blocks",
					"description": "Methods for interacting with blocks.",
				},
				"Filters": map[string]any{
					"name":        "Filters",
					"description": "Methods for interacting with filters.",
				},
				"Receipts": map[string]any{
					"name":        "Receipts",
					"description": "Methods for interacting with receipts.",
				},
				"Transactions": map[string]any{
					"name":        "Transactions",
					"description": "Methods for interacting with transactions.",
				},
				"Calls": map[string]any{
					"name":        "Sending",
					"description": "Methods for calling transactions.",
				},
				"Shards": map[string]any{
					"name":        "Shards",
					"description": "Methods for interacting with shards.",
				},
				"System": map[string]any{
					"name":        "System",
					"description": "System methods.",
				},
			},
		},
	}

	outputFile, _ := os.Create("openrpc.json")

	encoder := json.NewEncoder(outputFile)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(openrpcSpec); err != nil {
		return fmt.Errorf("error encoding JSON: %w", err)
	}

	return nil
}

func generateComponentsFromFile(filename string) map[string]Component {
	components := make(map[string]Component)
	file, err := os.Open(filename)
	if err != nil {
		log.Error().Err(err).Str("file", filename).Msg("Failed to open file")
		panic(err)
	}
	defer file.Close()

	var currentComponent *Component
	var currentName string
	var inObjectComponent bool

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)

		if line == "" {
			continue
		}
		compMatch := regexp.
			MustCompile(`^\s*\/\/\s*@component\s+(\w+)\s+(\w+)\s+(\w+)\s+"([^"]+)"`).
			FindStringSubmatch(line)
		if len(compMatch) == 5 {
			name := compMatch[1]
			title := compMatch[2]
			typ := compMatch[3]
			description := compMatch[4]

			component := Component{
				Title:       title,
				Type:        typ,
				Description: description,
				Properties:  make(map[string]Component),
			}
			components[name] = component
			currentComponent = &component
			currentName = name

			inObjectComponent = typ == "object"
			continue
		}

		if inObjectComponent {
			propMatch := regexp.
				MustCompile(`^\s*\/\/\s*@componentprop\s+(\w+)\s+(\w+)\s+(\w+)\s+(true|false)\s+"([^"]+)"`).
				FindStringSubmatch(line)
			if len(propMatch) == 6 {
				propName := propMatch[1]
				propTitle := propMatch[2]
				propType := propMatch[3]
				required := propMatch[4] == "true"
				propDescription := propMatch[5]

				if currentComponent != nil {
					property := Component{
						Title:       propTitle,
						Type:        propType,
						Description: propDescription,
					}
					currentComponent.Properties[propName] = property
					if required {
						currentComponent.Required = append(currentComponent.Required, propName)
					}
					components[currentName] = *currentComponent
				}
			} else {
				inObjectComponent = false
			}
		}
	}
	return components
}

func generateMethodsFromFile(filename string) []Method {
	methods := []Method{}
	file, err := os.Open(filename)
	if err != nil {
		log.Error().Err(err).Str("file", filename).Msg("Failed to open file")
		panic(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "/*") {
			method := Method{
				Params:   []Param{},
				Errors:   []any{},
				Examples: []any{},
			}
			for scanner.Scan() {
				line = scanner.Text()
				line = strings.TrimSpace(line)

				if strings.HasSuffix(line, "*/") {
					break
				}

				nameMatch := regexp.MustCompile(`@name\s+(\w+)`).FindStringSubmatch(line)
				if len(nameMatch) == 2 {
					method.Name = nameMatch[1]
				}

				summaryMatch := regexp.MustCompile(`@summary\s+(.+)`).FindStringSubmatch(line)
				if len(summaryMatch) == 2 {
					method.Summary = summaryMatch[1]
				}

				descriptionMatch := regexp.MustCompile(`@description\s*(.*)`).FindStringSubmatch(line)
				if len(descriptionMatch) == 2 {
					method.Description = descriptionMatch[1]
				}

				tagsMatch := regexp.MustCompile(`@tags\s+\[(.+)\]`).FindStringSubmatch(line)
				if len(tagsMatch) == 2 {
					tagNames := strings.Split(tagsMatch[1], ",")
					for _, tagName := range tagNames {
						method.Tags = append(method.Tags, TagRef{
							Ref: "#/components/tags/" + strings.TrimSpace(tagName),
						})
					}
				}

				paramMatch := regexp.MustCompile(`@param\s+(\w+)\s+(\w+)`).FindStringSubmatch(line)
				if len(paramMatch) == 3 {
					param := Param{
						Name: paramMatch[1],
						Schema: SchemaRef{
							Ref: "#/components/schemas/" + paramMatch[2],
						},
					}
					method.Params = append(method.Params, param)
				}

				returnsMatch := regexp.MustCompile(`@returns\s+(\w+)\s+(\w+)`).FindStringSubmatch(line)
				if len(returnsMatch) == 3 {
					method.Result = Result{
						Name: returnsMatch[1],
						Schema: SchemaRef{
							Ref: "#/components/schemas/" + returnsMatch[2],
						},
					}
				}
			}
			methods = append(methods, method)
		}
	}

	return methods
}
