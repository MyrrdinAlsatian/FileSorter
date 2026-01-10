package detector

import (
	"bytes"
	// "os"
	// "strings"
	"unicode"
)

func trimLeftSpaces(buf []byte) []byte {
	return bytes.TrimLeftFunc(buf, unicode.IsSpace)
}

func containsAll(buf []byte, patterns ...[]byte) bool {
	for _, pattern := range patterns {
		if !bytes.Contains(buf, pattern) {
			return false
		}
	}
	return true
}

// func detectPattern(path string) string {
func detectPattern(buf []byte) string {

	if len(buf) == 0 {
		return "empty"
	}

	if bytes.IndexByte(buf, 0) != -1 {
		return "binary"
	}

	for _, matcher := range patternMatchers {
		if ext, matched := matcher(buf); matched {
			return ext
		}
	}

	return "other extension"
}

func matchGo(buf []byte) (string, bool) {
	b := trimLeftSpaces(buf)
	return "go", bytes.HasPrefix(b, []byte("package ")) && containsAll(b, []byte("func "), []byte("import "))
}

func matchJs(buf []byte) (string, bool) {
	return "js", containsAll(buf, []byte("function"), []byte("{")) || bytes.Contains(buf, []byte("=> "))
}

func matchTs(buf []byte) (string, bool) {
	return "ts", containsAll(buf, []byte("interface"), []byte("type ")) || bytes.Contains(buf, []byte("export "))
}

func matchIni(buf []byte) (string, bool) {
	b := trimLeftSpaces(buf)
	return "ini", bytes.Contains(b, []byte("[")) && bytes.Contains(b, []byte("]")) && bytes.Contains(b, []byte("="))
}

func matchToml(buf []byte) (string, bool) {
	b := trimLeftSpaces(buf)
	return "toml", bytes.Contains(b, []byte("[")) && bytes.Contains(b, []byte("]")) && bytes.Contains(b, []byte("=")) && bytes.Contains(b, []byte("\""))
}

func matchLatex(buf []byte) (string, bool) {
	b := trimLeftSpaces(buf)
	return "latex", bytes.HasPrefix(b, []byte("\\documentclass"))
}

func matchPython(buf []byte) (string, bool) {
	b := trimLeftSpaces(buf)
	return "python", bytes.HasPrefix(b, []byte("#!/usr/bin/python")) || containsAll(b, []byte("def "), []byte(":"))
}

func matchPhp(buf []byte) (string, bool) {
	b := trimLeftSpaces(buf)
	return "php", bytes.HasPrefix(b, []byte("<?php"))
}

func matchCss(buf []byte) (string, bool) {
	return "css", containsAll(buf, []byte("{"), []byte("}")) && (bytes.Contains(buf, []byte("color")) || bytes.Contains(buf, []byte("background")) || bytes.Contains(buf, []byte("font")))
}
func matchJson(buf []byte) (string, bool) {
	b := trimLeftSpaces(buf)
	return "json", (len(b) > 0 && (b[0] == '{' || b[0] == '[') && bytes.Contains(b, []byte(":")))
}

func matchMd(buf []byte) (string, bool) {
	b := trimLeftSpaces(buf)
	return "md", bytes.HasPrefix(b, []byte("# ")) || bytes.Contains(b, []byte("\n#"))
}

var patternMatchers = []func([]byte) (string, bool){
	matchPhp,
	matchGo,
	matchPython,
	matchJson,
	matchMd,
	matchJs,
	matchTs,
	matchCss,
	matchIni,
	matchToml,
	matchLatex,
}
