package server

import (
	"io"

	"github.com/ghodss/yaml"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
)

type yamlMarshaller struct{}

var jsonMarshaller = &runtime.JSONPb{}

func (m *yamlMarshaller) Marshal(v interface{}) ([]byte, error) {
	b, err := jsonMarshaller.Marshal(v)
	if err != nil {
		return nil, err
	}
	return yaml.JSONToYAML(b)
}

func (m *yamlMarshaller) Unmarshal(data []byte, v interface{}) error {
	b, err := yaml.YAMLToJSON(data)
	if err != nil {
		return err
	}
	return jsonMarshaller.Unmarshal(b, v)
}

func (m *yamlMarshaller) NewDecoder(r io.Reader) runtime.Decoder {
	return jsonMarshaller.NewDecoder(r)
}

func (m *yamlMarshaller) NewEncoder(w io.Writer) runtime.Encoder {
	return jsonMarshaller.NewEncoder(w)
}

func (m *yamlMarshaller) ContentType() string {
	return "application/yaml"
}
