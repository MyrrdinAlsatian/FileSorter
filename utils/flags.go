package utils

import "flag"

type Options struct {
	ExportPath string
	SourceDir  string
}

func ParseFlags() Options {
	export := flag.String("export", "scan_results.json", "Path to export the scan results")
	source := flag.String("source", ".", "Source directory to scan")

	flag.Parse()

	return Options{
		ExportPath: *export,
		SourceDir:  *source,
	}
}
