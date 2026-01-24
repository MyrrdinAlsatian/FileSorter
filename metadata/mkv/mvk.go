// Package mkv extrait les métadonnées des fichiers vidéo Matroska (.mkv).
//
// CONCEPT : Parsing binaire avec recherche de patterns
// =====================================================
// Matroska utilise un format similaire à MP4 : des éléments (elements) imbriqués.
// Chaque élément a :
// - Un identifiant d'élément (1-4 bytes)
// - Une taille codée en EBML (variable length)
// - Le contenu (données brutes ou sous-éléments)
//
// DIFFÉRENCE AVEC MP4 :
// - Identifiants de taille variable (vs 4 bytes fixes en MP4)
// - Taille codée en EBML Variable Integer (vs simple big-endian)
// - Format ouvert et extensible
//
// STRUCTURE SIMPLIFIÉE D'UN MKV :
// ┌─ EBML Header (info sur le format)
// ├─ Segment
// │  ├─ SeekHead
// │  ├─ Info (métadonnées générales)
// │  │  ├─ TimecodeScale
// │  │  ├─ Duration
// │  │  └─ Title (ce qu'on cherche)
// │  ├─ Tracks (description des pistes audio/vidéo)
// │  └─ Cluster (données vidéo)
// └─ Cues (index)
//
// Cette fonction cherche principalement le titre dans la section Info.
package mkv

import "bytes"

// Identifiants d'éléments Matroska (en hexadécimal)
var (
	mkvSegment = []byte{0x18, 0x53, 0x80, 0x67} // Segment Header (conteneur principal)
	mkvInfo    = []byte{0x15, 0x49, 0xA9, 0x66} // Info Header (métadonnées)
	mkvDate    = []byte{0x44, 0x89}             // Date UTC (timestamp)
	mkvTile    = []byte{0x7B, 0xA9}             // Title (titre du fichier)
)

// Parse extrait le titre d'un fichier Matroska (MKV).
//
// PROCESSUS SIMPLIFIÉ :
// 1. Chercher le "Segment" (conteneur principal)
// 2. À l'intérieur, chercher l'élément "Info" (métadonnées)
// 3. À l'intérieur de Info, chercher l'élément "Title"
// 4. Extraire la chaîne de caractères du titre
//
// LIMITATION :
// - Cette fonction cherche simplement la première occurrence
// - Elle ne valide pas la structure complète du fichier
// - Elle est tolérante aux erreurs (retourne vide si non trouvé)
//
// UTILISATION :
// Cette fonction est appelée depuis metadata/utils.go pour les fichiers .mkv
// dans GetFileMeta().
//
// Paramètres :
//   - buf : contenu du fichier MKV (ou au moins les premiers KB)
//
// Retour :
//   - string : le titre trouvé (ou chaîne vide si non trouvé)
//   - bool : true si un titre valide a été trouvé et extrait
//
// Exemple :
//
//	title, found := mkv.Parse(fileContent)
//	if found {
//	    fmt.Println("Titre du vidéo :", title)
//	}
func Parse(buf []byte) (string, bool) {
	// Chercher la boîte Segment dans le buffer
	// bytes.Index retourne l'indice de la première occurrence (ou -1 si non trouvé)
	s := bytes.Index(buf, mkvSegment)

	// Si le segment n'est pas trouvé, le fichier n'est pas un MKV valide
	if s == -1 {
		return "", false
	}

	// Continuer après l'en-tête du segment (skip les 4 bytes de l'identifiant)
	segment := buf[s+4:]

	// Chercher la section Info dans le segment
	i := bytes.Index(segment, mkvInfo)
	if i == -1 {
		return "", false
	}

	// Continuer après l'identifiant Info
	info := segment[i+4:]

	// Chercher l'élément Title dans la section Info
	t := bytes.Index(info, mkvTile)
	if t == -1 {
		return "", false
	}

	// Continuer après l'identifiant Title
	titleSection := info[t+2:]

	// Lire la taille de la chaîne de titre en format EBML Variable Integer
	// readEBMLSize retourne (taille, nombre de bytes lus)
	size, sizeLen := readEBMLSize(titleSection)

	// Vérifications de sécurité
	if sizeLen == 0 || size+sizeLen > len(titleSection) {
		return "", false
	}

	// Extraire la chaîne de titre
	start := sizeLen
	end := sizeLen + int(size)

	if end > len(titleSection) {
		return "", false
	}

	// Nettoyer les octets null (terminateurs UTF-8)
	titleBytes := bytes.Trim(titleSection[start:end], "\x00")

	return string(titleBytes), true
}

// readEBMLSize décode une taille EBML à partir d'un slice d'octets.
//
// CONCEPT : Variable-Length Integer (VLI) Encoding
// ==================================================
// EBML utilise un codage de longueur variable pour les tailles.
// C'est plus efficace que de toujours utiliser 4 ou 8 bytes.
//
// COMMENT ÇA MARCHE :
// Le premier octet indique le nombre d'octets total :
// - Si MSB (Most Significant Bit) = 1 : 1 octet total
// - Si les 2 premiers bits = 01 : 2 octets total
// - Si les 3 premiers bits = 001 : 3 octets total
// - Si les 4 premiers bits = 0001 : 4 octets total
// etc.
//
// EXEMPLES :
// - 0x8F : 1 octet, valeur = 0x0F = 15
// - 0x40 0x00 : 2 octets, valeur = 0x00 = 0
// - 0x20 0x00 0x00 : 3 octets, valeur = 0x00 = 0
//
// POURQUOI ?
// - Les petites tailles utilisent 1 byte (< 127)
// - Les moyennes utilisent 2-3 bytes
// - Économise de l'espace par rapport à toujours 4-8 bytes
//
// Paramètres :
//   - data : slice contenant les octets à décoder (au moins 1 byte)
//
// Retour :
//   - int : la valeur numérique de la taille décodée
//   - int : le nombre d'octets consommés (1-8)
//     retour (0, 0) en cas d'erreur
//
// Erreurs gérées :
//   - data vide
//   - longueur calculée > 8 octets (invalide EBML)
//   - données insuffisantes dans le buffer
func readEBMLSize(data []byte) (int, int) {
	if len(data) == 0 {
		return 0, 0
	}

	b := data[0]
	mask := byte(0x80) // Commence par le MSB
	length := 1

	// Trouver le nombre d'octets : compter les bits zéro au début
	for (b & mask) == 0 {
		mask >>= 1 // Décaler le mask vers la droite
		length++
	}

	// Valider la longueur
	if length > 8 || length > len(data) {
		return 0, 0
	}

	// Extraire la valeur : les bits de poids fort du premier byte plus les bytes suivants
	val := int(b & (mask - 1))
	for i := 1; i < length; i++ {
		// Shift existing value left by 8 bits, then add the next byte
		val = (val << 8) | int(data[i])
	}

	return val, length
}
