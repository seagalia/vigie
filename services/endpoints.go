package services

import (
	"bytes"
	"encoding/json"

	eds "github.com/seagalia/vigie/sdk/vigie/eds/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

func marshalEndpointsJSON(endpoints []*eds.Endpoint) ([]byte, error) {
	marshaler := protojson.MarshalOptions{UseProtoNames: true, EmitUnpopulated: true}
	items := make([]json.RawMessage, 0, len(endpoints))
	for _, endpoint := range endpoints {
		item, err := marshaler.Marshal(endpoint)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	compact, err := json.Marshal(items)
	if err != nil {
		return nil, err
	}
	var indented bytes.Buffer
	err = json.Indent(&indented, compact, "", "  ")
	if err != nil {
		return nil, err
	}
	indented.WriteByte('\n')
	return indented.Bytes(), nil
}
