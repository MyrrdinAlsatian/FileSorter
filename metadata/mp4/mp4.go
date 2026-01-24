package mp4

import (
	"encoding/binary"
	"io"
	"os"
	"time"
)

type MP4Metadata struct {
	Title    string        `json:"title,omitempty"`
	Width    int           `json:"width,omitempty"`
	Height   int           `json:"height,omitempty"`
	Duration time.Duration `json:"duration,omitempty"`
	Date     time.Time     `json:"date,omitempty"`
	Valid    bool          `json:"valid"`
}

const (
	boxMvhd = "mvhd"
	boxTkhd = "tkhd"
	boxCnam = "@nam"
)

func Parse(path string) (*MP4Metadata, error) {
	// Implémentation fictive pour l'exemple

	f, err := os.Open(path)
	if err != nil {
		return &MP4Metadata{Valid: false}, err
	}
	defer f.Close()

	meta := &MP4Metadata{}

	for {
		var size uint32
		var boxType [4]byte

		if err := binary.Read(f, binary.BigEndian, &size); err != nil {
			return meta, nil
		}

		if _, err := io.ReadFull(f, boxType[:]); err != nil {
			return meta, nil
		}

		switch string(boxType[:]) {
		case boxMvhd:
			// Lire les données mvhd pour la durée
			ParseMvhd(f, meta, size)
		case boxTkhd:
			// Lire les données tkhd pour la largeur et la hauteur

			ParseTkhd(f, meta, size)
		default:
			// Ignorer les autres boîtes
			if _, err := f.Seek(int64(size-8), io.SeekCurrent); err != nil {
				return meta, nil
			}
		}
	}
}

func ParseMvhd(r io.Reader, meta *MP4Metadata, size uint32) {
	var versionAndFlags [1]byte
	r.Read(versionAndFlags[:])
	// Skip the next 3 bytes of flags
	skip := make([]byte, 3)
	io.ReadFull(r, skip)

	if versionAndFlags[0] == 0 {
		var ctime, mtime, timescale, duration uint32

		binary.Read(r, binary.BigEndian, &ctime)
		binary.Read(r, binary.BigEndian, &mtime)
		binary.Read(r, binary.BigEndian, &timescale)
		binary.Read(r, binary.BigEndian, &duration)

		meta.Duration = time.Duration(duration) * time.Second / time.Duration(timescale)
		meta.Date = time.Unix(int64(ctime-2_082_844_800), 0)
		meta.Valid = true
	} else {
		skip := make([]byte, size-8-4)
		io.ReadFull(r, skip)
	}
}

func ParseTkhd(r io.Reader, meta *MP4Metadata, size uint32) {
	skip := make([]byte, 76)
	io.ReadFull(r, skip)

	var widthFixed, heightFixed uint32
	binary.Read(r, binary.BigEndian, &widthFixed)
	binary.Read(r, binary.BigEndian, &heightFixed)

	meta.Width = int(widthFixed >> 16)
	meta.Height = int(heightFixed >> 16)
}
