package mkv

import "bytes"

var (
	mkvSegment = []byte{0x18, 0x53, 0x80, 0x67} // Segment Header
	mkvInfo    = []byte{0x15, 0x49, 0xA9, 0x66} // Info Header
	mkvDate    = []byte{0x44, 0x89}             // Date UTC
	mkvTile    = []byte{0x7B, 0xA9}             // Title
)

func Parse(buf []byte) (string, bool) {
	// Rechercher le segment MKV
	s := bytes.Index(buf, mkvSegment)

	// Si le segment n'est pas trouvé, retourner vide
	if s == -1 {
		return "", false
	}

	segment := buf[s+4:]

	// Rechercher la section Info
	i := bytes.Index(segment, mkvInfo)
	if i == -1 {
		return "", false
	}

	info := segment[i+4:]
	// Rechercher le titre
	t := bytes.Index(info, mkvTile)
	if t == -1 {
		return "", false
	}

	titleSection := info[t+2:]

	size, sizeLen := readEBMLSize(titleSection)

	if sizeLen == 0 || size+sizeLen > len(titleSection) {
		return "", false
	}

	start := sizeLen
	end := sizeLen + int(size)

	if end > len(titleSection) {
		return "", false
	}

	titleBytes := bytes.Trim(titleSection[start:end], "\x00") // Nettoyer les null bytes

	return string(titleBytes), true
}

// readEBMLSize décode une taille EBML à partir d'un slice d'octets.
//
// La taille EBML est un format de longueur variable utilisé dans les conteneurs Matroska.
// Le premier octet indique la longueur totale : son bit de poids fort détermine combien d'octets
// composent la taille. Les bits restants du premier octet font partie de la valeur.
//
// Par exemple :
//   - 0x8X : taille codée sur 1 octet (bit de poids fort = 1)
//   - 0x4X XX : taille codée sur 2 octets (bit de poids fort du premier = 0, suivant = 1)
//   - 0x2X XX XX : taille codée sur 3 octets
//
// Paramètres :
//
//	data : slice contenant les octets à décoder
//
// Retour :
//
//	val : la valeur numérique de la taille décodée
//	length : le nombre d'octets consommés pour cette taille
//
// Cas d'erreur :
// La fonction retourne (0, 0) si :
//   - data est vide
//   - la longueur calculée dépasse 8 octets
//   - la longueur calculée dépasse la taille disponible dans data
func readEBMLSize(data []byte) (int, int) {
	if len(data) == 0 {
		return 0, 0
	}

	b := data[0]
	mask := byte(0x80)
	length := 1

	for (b & mask) == 0 {
		mask >>= 1
		length++
	}
	if length > 8 || length > len(data) {
		return 0, 0 // Taille invalide
	}

	val := int(b & (mask - 1))
	for i := 1; i < length; i++ {
		val = (val << 8) | int(data[i])
	}

	return val, length
}
