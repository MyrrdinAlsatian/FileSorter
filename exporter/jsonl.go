package exporter

import (
	"FileRecoveryOrganizer/types"
	"bufio"
	"encoding/json"
	"os"
)

type JSONExporter struct {
	writer *bufio.Writer
	file   *os.File
}

func NewJSONExporter(filePath string) (*JSONExporter, error) {
	file, err := os.Create(filePath)
	if err != nil {
		return nil, err
	}

	return &JSONExporter{
		file:   file,
		writer: bufio.NewWriterSize(file, 64*1024), // 64KB buffer
	}, nil
}

func (je *JSONExporter) Write(result types.Result) error {
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}

	_, err = je.writer.Write(data)
	if err != nil {
		return err
	}

	if err := je.writer.WriteByte('\n'); err != nil {
		return err
	}

	return nil
}

func (je *JSONExporter) Close() error {
	if err := je.writer.Flush(); err != nil {
		je.file.Close()
		return err
	}
	return je.file.Close()
}
