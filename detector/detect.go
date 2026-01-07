package detector

func Detect(path string) string {
	typeOrExt := detectFileType(path)
	if typeOrExt != "other extension" {
		return typeOrExt
	}

	patternType := detectPattern(path)

	if patternType != "other extension" {
		return patternType
	}

	return "unknown"
}
