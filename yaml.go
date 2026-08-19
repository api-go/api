package api

import (
	"bytes"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// ParseActionYAML parses a data-driven Action definition without applying
// product-specific defaults or registering it globally.
func ParseActionYAML(source string) (map[string]any, error) {
	definition := map[string]any{}
	if err := yaml.Unmarshal([]byte(source), &definition); err != nil {
		return nil, fmt.Errorf("invalid Action YAML: %w", err)
	}
	if definition == nil {
		return nil, fmt.Errorf("Action YAML must be an object")
	}
	return definition, nil
}

// PatchActionTokens replaces the top-level tokens field while preserving the
// rest of the YAML document, including comments and field order.
func PatchActionTokens(source string, tokens []string) (string, error) {
	document, root, err := parseActionYAMLDocument(source)
	if err != nil {
		return "", err
	}
	index := yamlMappingIndex(root, "tokens")
	if len(tokens) == 0 {
		if index >= 0 {
			root.Content = append(root.Content[:index], root.Content[index+2:]...)
		}
	} else {
		sequence := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Style: yaml.FlowStyle}
		for _, token := range tokens {
			sequence.Content = append(sequence.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: token})
		}
		if index >= 0 {
			root.Content[index+1] = sequence
		} else {
			root.Content = append(root.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "tokens"}, sequence)
		}
	}
	return encodeActionYAML(document)
}

// ConvertActionJSONToYAML converts a legacy JSON Action document to readable
// block-style YAML. It does not add defaults or modify the Action semantics.
func ConvertActionJSONToYAML(source string) (string, error) {
	document, root, err := parseActionYAMLDocument(source)
	if err != nil {
		return "", err
	}
	useYAMLBlockStyle(root)
	root.HeadComment = "API Action configuration. Field order and comments are preserved."
	return encodeActionYAML(document)
}

func parseActionYAMLDocument(source string) (*yaml.Node, *yaml.Node, error) {
	var document yaml.Node
	if err := yaml.Unmarshal([]byte(source), &document); err != nil {
		return nil, nil, fmt.Errorf("invalid Action YAML: %w", err)
	}
	if len(document.Content) == 0 || document.Content[0].Kind != yaml.MappingNode {
		return nil, nil, fmt.Errorf("Action YAML must be an object")
	}
	return &document, document.Content[0], nil
}

func encodeActionYAML(document *yaml.Node) (string, error) {
	var output bytes.Buffer
	encoder := yaml.NewEncoder(&output)
	encoder.SetIndent(2)
	if err := encoder.Encode(document); err != nil {
		return "", err
	}
	_ = encoder.Close()
	return strings.TrimRight(output.String(), "\n") + "\n", nil
}

func yamlMappingIndex(root *yaml.Node, key string) int {
	for index := 0; index+1 < len(root.Content); index += 2 {
		if root.Content[index].Value == key {
			return index
		}
	}
	return -1
}

func useYAMLBlockStyle(node *yaml.Node) {
	if node.Kind == yaml.MappingNode || node.Kind == yaml.SequenceNode {
		node.Style = 0
	}
	if node.Kind == yaml.ScalarNode {
		node.Style = 0
	}
	if node.Kind == yaml.MappingNode {
		for index := 0; index < len(node.Content); index += 2 {
			node.Content[index].Style = 0
		}
	}
	for _, child := range node.Content {
		useYAMLBlockStyle(child)
	}
}
