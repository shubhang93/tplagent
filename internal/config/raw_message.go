package config

import (
	"encoding/json"
	"gopkg.in/yaml.v3"
)

type RawMessage struct {
	yamlNode    *yaml.Node
	jsonMessage json.RawMessage
}

func (r *RawMessage) MarshalYAML() (interface{}, error) {
	return r.yamlNode, nil
}

func (r *RawMessage) MarshalJSON() ([]byte, error) {
	return r.jsonMessage, nil
}

func (r *RawMessage) UnmarshalYAML(value *yaml.Node) error {
	r.yamlNode = value
	return nil
}

func (r *RawMessage) UnmarshalJSON(bytes []byte) error {
	r.jsonMessage = bytes
	return nil
}

func NewJSONRawMessage(msg json.RawMessage) RawMessage {
	return RawMessage{
		jsonMessage: msg,
	}
}

func NewYAMLRawMessage(msg *yaml.Node) RawMessage {
	return RawMessage{
		yamlNode: msg,
	}
}

func (r RawMessage) Decode(v any) error {
	if r.yamlNode != nil {
		return r.yamlNode.Decode(v)
	}
	return json.Unmarshal(r.jsonMessage, v)
}
