//go:build ignore
// +build ignore

// Outil de debug pour tester le parser MKV
// Usage: go run debug_mkv.go /chemin/vers/fichier.mkv
package main

import (
	"encoding/hex"
	"fmt"
	"os"

	"FileRecoveryOrganizer/metadata/mkv"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run debug_mkv.go <fichier.mkv>")
		os.Exit(1)
	}

	path := os.Args[1]
	fmt.Printf("=== Debug MKV Parser ===\n")
	fmt.Printf("Fichier: %s\n\n", path)

	// Ouvrir le fichier
	f, err := os.Open(path)
	if err != nil {
		fmt.Printf("Erreur ouverture: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	// Lire les premiers bytes
	buf := make([]byte, mkv.ScanSize)
	n, err := f.Read(buf)
	if err != nil {
		fmt.Printf("Erreur lecture: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Bytes lus: %d\n", n)

	// Afficher les premiers 64 bytes en hex pour voir la signature
	fmt.Printf("\nPremiers 64 bytes (hex):\n")
	fmt.Println(hex.Dump(buf[:min(64, n)]))

	// Chercher la signature EBML (0x1A 0x45 0xDF 0xA3)
	ebmlSignature := []byte{0x1A, 0x45, 0xDF, 0xA3}
	found := false
	for i := 0; i < min(100, n-4); i++ {
		if buf[i] == ebmlSignature[0] &&
			buf[i+1] == ebmlSignature[1] &&
			buf[i+2] == ebmlSignature[2] &&
			buf[i+3] == ebmlSignature[3] {
			fmt.Printf("\n✅ Signature EBML trouvée à l'offset %d\n", i)
			found = true
			break
		}
	}
	if !found {
		fmt.Println("\n❌ Signature EBML non trouvée - Ce n'est peut-être pas un vrai MKV")
	}

	// Chercher le Segment (0x18 0x53 0x80 0x67)
	segmentSignature := []byte{0x18, 0x53, 0x80, 0x67}
	for i := 0; i < min(1000, n-4); i++ {
		if buf[i] == segmentSignature[0] &&
			buf[i+1] == segmentSignature[1] &&
			buf[i+2] == segmentSignature[2] &&
			buf[i+3] == segmentSignature[3] {
			fmt.Printf("✅ Segment trouvé à l'offset %d\n", i)
			break
		}
	}

	// Chercher Info (0x15 0x49 0xA9 0x66)
	infoSignature := []byte{0x15, 0x49, 0xA9, 0x66}
	for i := 0; i < min(50000, n-4); i++ {
		if buf[i] == infoSignature[0] &&
			buf[i+1] == infoSignature[1] &&
			buf[i+2] == infoSignature[2] &&
			buf[i+3] == infoSignature[3] {
			fmt.Printf("✅ Info trouvé à l'offset %d\n", i)

			// Afficher les bytes autour pour debug
			start := max(0, i-4)
			end := min(n, i+100)
			fmt.Printf("\nContenu autour de Info (offset %d):\n", i)
			fmt.Println(hex.Dump(buf[start:end]))
			break
		}
	}

	// Chercher Title (0x7B 0xA9)
	titleSignature := []byte{0x7B, 0xA9}
	for i := 0; i < min(100000, n-2); i++ {
		if buf[i] == titleSignature[0] && buf[i+1] == titleSignature[1] {
			fmt.Printf("✅ Potentiel Title trouvé à l'offset %d\n", i)

			// Afficher le contexte
			start := max(0, i-4)
			end := min(n, i+50)
			fmt.Printf("\nContenu autour de Title (offset %d):\n", i)
			fmt.Println(hex.Dump(buf[start:end]))
			break
		}
	}

	// Chercher Tags (0x12 0x54 0xC3 0x67)
	tagsSignature := []byte{0x12, 0x54, 0xC3, 0x67}
	for i := 0; i < min(500000, n-4); i++ {
		if buf[i] == tagsSignature[0] &&
			buf[i+1] == tagsSignature[1] &&
			buf[i+2] == tagsSignature[2] &&
			buf[i+3] == tagsSignature[3] {
			fmt.Printf("✅ Tags trouvé à l'offset %d\n", i)
			break
		}
	}

	// Maintenant tester le parser
	fmt.Println("\n=== Test du Parser ===")
	meta := mkv.ParseFile(buf[:n], path)
	fmt.Printf("Title:  %q\n", meta.Title)
	fmt.Printf("Source: %q\n", meta.Source)
	fmt.Printf("Date:   %v\n", meta.Date)
	fmt.Printf("Valid:  %v\n", meta.Valid)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
