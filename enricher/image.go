package enricher

import (
	"image"
	"os"

	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	"FileRecoveryOrganizer/types"

	"github.com/rwcarlsen/goexif/exif"
)

func EnrichImage(res *types.Result) {
	f, err := os.Open(res.Path)
	if err != nil {
		return
	}
	defer f.Close()

	x, err := exif.Decode(f)
	if err != nil {
		return
	}

	meta := &types.ImageMeta{
		HasExif: true,
	}

	if cfg, _, err := image.DecodeConfig(f); err == nil {
		meta.Width = cfg.Width
		meta.Height = cfg.Height
	}
	_, _ = f.Seek(0, 0) // Reset file pointer

	iexif := &types.ImageExif{}

	if camModel, err := x.Get(exif.Model); err == nil {
		iexif.CameraModel, _ = camModel.StringVal()
	}

	if dt, err := x.Get(exif.DateTimeOriginal); err == nil {
		iexif.DateTaken, _ = dt.StringVal()
	}

	if lat, lon, err := x.LatLong(); err == nil {
		iexif.GPSLatitude = lat
		iexif.GPSLongitude = lon
	}

	meta.Exif = iexif
	res.Image = meta
}
