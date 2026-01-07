package detector

import (
	"os"
	"strings"
)

func detectPattern(path string) string {

	f, err := os.Open(path)
	if err != nil {
		return "Error opening file"
	}
	defer f.Close()

	buf := make([]byte, 1024)
	n, err := f.Read(buf)
	if err != nil {
		return "Error reading file"
	}

	content := string(buf[:n])
	content = strings.TrimSpace(content)
	switch {
	case strings.HasPrefix(content, "package ") || strings.Contains(content, "func ") || strings.Contains(content, "import "):
		return "go"
	case strings.HasPrefix(content, "<?php"):
		return "php"
	case strings.HasPrefix(content, "#!/usr/bin/python") || strings.Contains(content, "def "):
		return "python"
	case strings.HasPrefix(content, "#!/bin/bash") || strings.Contains(content, "echo ") || strings.Contains(content, "function "):
		return "bash"
	case strings.HasPrefix(content, "#!/usr/bin/env ruby") || strings.Contains(content, "def ") || strings.Contains(content, "class "):
		return "ruby"
	case strings.HasPrefix(content, "#!/usr/bin/perl"):
		return "perl"
	case strings.Contains(content, "function") || strings.Contains(content, "var ") || strings.Contains(content, "let ") ||
		strings.Contains(content, "const ") || strings.Contains(content, "=>"):
		return "js"
	case strings.Contains(content, "interface") || strings.Contains(content, "type ") || strings.Contains(content, "export "):
		return "ts"
	}
	// ---------- Documents / Web ----------
	switch {
	case strings.HasPrefix(content, "{") || strings.HasPrefix(content, "["):
		return "json"
	case strings.HasPrefix(content, "<?xml"):
		return "xml"
	case strings.HasPrefix(content, "#") || strings.Contains(content, "\n#"):
		return "md"
	case strings.HasPrefix(content, "<!DOCTYPE html>") || strings.HasPrefix(content, "<html"):
		return "html"
	case strings.HasPrefix(content, "\\documentclass"):
		return "latex"
	case strings.Contains(content, "{") && strings.Contains(content, "}") &&
		(strings.Contains(content, "color") || strings.Contains(content, "background") || strings.Contains(content, "font")):
		return "css"
	}

	// ---------- Fichiers de config ----------
	switch {
	case strings.Contains(content, "---") && strings.Contains(content, ":"):
		return "yaml"
	case strings.Contains(content, "[") && strings.Contains(content, "]") && strings.Contains(content, "="):
		return "ini"
	case strings.Contains(content, "[") && strings.Contains(content, "]") && strings.Contains(content, "="):
		return "toml"
	}
	return "other extension"
}
