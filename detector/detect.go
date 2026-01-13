package detector

func Detect(path string) string {
	typeOrExt := detectFileType(path)
	if typeOrExt == "" {
		return "unknown"
	}
	if typeOrExt != "other extension" {
		return typeOrExt
	}

	patternType := detectPattern([]byte(path))

	if patternType != "other extension" {
		return patternType
	}
	return "unknown"
}
