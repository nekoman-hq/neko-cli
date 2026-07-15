package pluginindex

import (
	"bytes"
	"encoding/json"
	"io"
)

type jsonPluginIndexOutputBuilder struct{}

func (jsonPluginIndexOutputBuilder) Build(index *Index, options WriteOptions) ([]byte, error) {
	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	if options.Pretty {
		encoder.SetIndent("", "  ")
	}
	if err := encoder.Encode(index); err != nil {
		return nil, err
	}
	return append([]byte(nil), output.Bytes()...), nil
}

// Write serializes an index as stable, pretty JSON.
func Write(index *Index, writer io.Writer) error {
	return WriteWithOptions(index, writer, WriteOptions{Pretty: true})
}

// WriteWithOptions serializes an index as JSON.
func WriteWithOptions(index *Index, writer io.Writer, options WriteOptions) error {
	output, err := (jsonPluginIndexOutputBuilder{}).Build(index, options)
	if err != nil {
		return err
	}
	_, err = writer.Write(output)
	return err
}
