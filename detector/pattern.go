package detector

import (
	"bytes"
	"unicode"
)

// trimLeftSpaces supprime les espaces et caractères whitespace du début du buffer.
//
// bytes.TrimLeftFunc applique une fonction sur chaque byte du début
// tant que la fonction retourne true.
// unicode.IsSpace retourne true pour les espaces, tabs, newlines, etc.
//
// Exemple :
//
//	"  hello" -> "hello"
//	"\n\t json" -> "json"
func trimLeftSpaces(buf []byte) []byte {
	return bytes.TrimLeftFunc(buf, unicode.IsSpace)
}

// containsAll vérifie que le buffer contient TOUS les patterns fournis.
//
// Cette fonction est une abstraction utile pour les vérifications de contenu.
// Par exemple, un fichier JSON valide doit avoir { et :
//
// Paramètres :
//   - buf : le contenu du fichier en bytes
//   - patterns : ensemble de patterns à chercher
//
// Retour :
//   - bool : true si TOUS les patterns sont trouvés, false sinon
func containsAll(buf []byte, patterns ...[]byte) bool {
	for _, pattern := range patterns {
		if !bytes.Contains(buf, pattern) {
			return false
		}
	}
	return true
}

// detectPattern essaie d'identifier le type de fichier en analysant son contenu.
//
// CONCEPT : Pattern matching pour fichiers texte
// Cette fonction cherche des chaînes/patterns caractéristiques dans le contenu
// pour identifier le langage ou le format :
// - "package " + "type " + "func " + "import " -> Go
// - "function" + "{" ou "=> " -> JavaScript
// - "#!/usr/bin/python" ou "def " + ":" -> Python
// etc.
//
// LIMITES :
// - Uniquement pour fichiers texte (vérification binaire en premier)
// - Peut avoir des faux positifs
// - Plus lent que la détection par magic numbers
//
// Paramètres :
//   - buf : contenu du fichier en bytes
//
// Retour :
//   - string : type détecté ou "other extension" si non trouvé
func detectPattern(buf []byte) string {

	if len(buf) == 0 {
		return "empty"
	}

	// Vérifier si c'est un fichier binaire (contient un byte null)
	// Les fichiers binaires ont des bytes à 0
	if bytes.IndexByte(buf, 0) != -1 {
		return "binary"
	}

	// Essayer chaque matcher (fonction de détection) en ordre
	// Les matchers sont définis dans la liste patternMatchers ci-dessous
	for _, matcher := range patternMatchers {
		if ext, matched := matcher(buf); matched {
			return ext
		}
	}

	// Pas d'identifiant trouvé
	return "other extension"
}

// Fonctions matchers pour différents langages/formats
// Chaque fonction retourne (extension, matched bool)

func matchGo(buf []byte) (string, bool) {
	b := trimLeftSpaces(buf)
	// Le Go doit avoir : package, type et func dans les imports
	return "go", bytes.HasPrefix(b, []byte("package ")) && bytes.Contains(b, []byte("type ")) && containsAll(b, []byte("func "), []byte("import "))
}

func matchJs(buf []byte) (string, bool) {
	// JavaScript : doit avoir function et {} OU arrow functions =>
	return "js", containsAll(buf, []byte("function"), []byte("{")) || bytes.Contains(buf, []byte("=> "))
}

func matchTs(buf []byte) (string, bool) {
	// TypeScript : doit avoir interface/type ou export
	return "ts", containsAll(buf, []byte("interface"), []byte("type ")) || bytes.Contains(buf, []byte("export "))
}

func matchIni(buf []byte) (string, bool) {
	// INI : doit avoir [sections] = clé:valeur
	b := trimLeftSpaces(buf)
	return "ini", bytes.Contains(b, []byte("[")) && bytes.Contains(b, []byte("]")) && bytes.Contains(b, []byte("="))
}

func matchToml(buf []byte) (string, bool) {
	// TOML : comme INI mais avec guillemets
	b := trimLeftSpaces(buf)
	return "toml", bytes.Contains(b, []byte("[")) && bytes.Contains(b, []byte("]")) && bytes.Contains(b, []byte("=")) && bytes.Contains(b, []byte("\""))
}

func matchLatex(buf []byte) (string, bool) {
	// LaTeX : commence par \documentclass
	b := trimLeftSpaces(buf)
	return "latex", bytes.HasPrefix(b, []byte("\\documentclass"))
}

func matchPython(buf []byte) (string, bool) {
	// Python : shebang #!/usr/bin/python OU def : (fonctions)
	b := trimLeftSpaces(buf)
	return "python", bytes.HasPrefix(b, []byte("#!/usr/bin/python")) || containsAll(b, []byte("def "), []byte(":"))
}

func matchPhp(buf []byte) (string, bool) {
	// PHP : commence par <?php
	b := trimLeftSpaces(buf)
	return "php", bytes.HasPrefix(b, []byte("<?php"))
}

func matchCss(buf []byte) (string, bool) {
	// CSS : doit avoir {} et propriétés CSS typiques
	return "css", containsAll(buf, []byte("{"), []byte("}")) && (bytes.Contains(buf, []byte("color")) || bytes.Contains(buf, []byte("background")) || bytes.Contains(buf, []byte("font")))
}

func matchJson(buf []byte) (string, bool) {
	// JSON : commence par { ou [ et contient :
	b := trimLeftSpaces(buf)
	return "json", (len(b) > 0 && (b[0] == '{' || b[0] == '[') && bytes.Contains(b, []byte(":")))
}

func matchMd(buf []byte) (string, bool) {
	// Markdown : commence par # ou contient # en début de ligne
	b := trimLeftSpaces(buf)
	return "md", bytes.HasPrefix(b, []byte("# ")) || bytes.Contains(b, []byte("\n#"))
}

// patternMatchers est la liste de toutes les fonctions de détection
// L'ordre compte : les plus spécifiques en premier
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
