// Package app orchestre les cas d'usage réutilisables de FileSorter.
package app

import (
	"context"
	"errors"

	"FileRecoveryOrganizer/exporter"
	"FileRecoveryOrganizer/scanner"
	"FileRecoveryOrganizer/types"
)

// ScanRequest regroupe les paramètres nécessaires au scan et à son export JSONL.
type ScanRequest struct {
	SourceDir   string
	ExportPath  string
	Append      bool
	Workers     int
	ScanOptions scanner.ScanOptions
	OnProgress  func()
	OnResult    func(types.Result)
}

// ScanOutcome contient les statistiques et les avertissements d'export du scan.
type ScanOutcome struct {
	Stats       *scanner.SafeStats
	ResultCount int
	ExportError error
}

// ScanAndExport exécute le scan parallèle et consomme tous ses résultats avant de retourner.
func ScanAndExport(ctx context.Context, request ScanRequest) (ScanOutcome, error) {
	var jsonExporter *exporter.JSONExporter
	var err error
	if request.Append {
		jsonExporter, err = exporter.NewJSONExporterAppend(request.ExportPath)
	} else {
		jsonExporter, err = exporter.NewJSONExporter(request.ExportPath)
	}
	if err != nil {
		return ScanOutcome{}, err
	}

	collector := scanner.NewCollector(2048)
	outcome := ScanOutcome{Stats: scanner.NewStats()}
	exportDone := make(chan struct{})

	// Le scan peut remplir un canal bufferisé; un consommateur concurrent évite de bloquer les workers.
	go func() {
		defer close(exportDone)
		for result := range collector.Results {
			if request.OnResult != nil {
				request.OnResult(result)
			}
			if err := jsonExporter.Write(result); err != nil {
				if outcome.ExportError == nil {
					outcome.ExportError = err
				}
			}
			outcome.ResultCount++
		}
	}()

	scanErr := scanner.ScanDirectoryParallelWithScanOptionsContext(
		ctx,
		request.SourceDir,
		collector,
		outcome.Stats,
		request.OnProgress,
		request.Workers,
		request.ScanOptions,
	)
	<-exportDone
	if closeErr := jsonExporter.Close(); closeErr != nil {
		outcome.ExportError = errors.Join(outcome.ExportError, closeErr)
	}

	return outcome, scanErr
}
