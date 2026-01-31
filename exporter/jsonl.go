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

func StreamJSONL(path string, results <-chan types.Result) error {
	f, error := os.Create(path)
	if error != nil {
		return error
	}
	defer f.Close()

	writer := bufio.NewWriterSize(f, 64*1024) // 64KB buffer
	defer writer.Flush()

	for result := range results {
		data, err := json.Marshal(result)
		if err != nil {
			continue
		}
		writer.Write(data)
		writer.WriteByte('\n')
	}

	return nil
}

func StreamJSONLWithFilter(path string, results <-chan types.Result, filter ...Filter) error {
	f, error := os.Create(path)
	if error != nil {
		return error
	}
	defer f.Close()

	writer := bufio.NewWriterSize(f, 64*1024) // 64KB buffer
	defer writer.Flush()

	for result := range results {
		skip := false
		for _, f := range filter {
			if !f(result) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}

		data, err := json.Marshal(result)
		if err != nil {
			continue
		}
		writer.Write(data)
		writer.WriteByte('\n')
	}

	return nil
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

// NewJSONExporterAppend crée un exporter qui ajoute au fichier existant.
// Utilisé pour le mode resume.
func NewJSONExporterAppend(filePath string) (*JSONExporter, error) {
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
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
