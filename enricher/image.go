package enricher

import (
	"os"

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

	meta := &types.ImageMetadata{}

	if camModel, err := x.Get(exif.Model); err == nil {
		meta.CameraModel, _ = camModel.StringVal()
	}

	if dt, err := x.Get(exif.DateTimeOriginal); err == nil {
		meta.DateTaken, _ = dt.StringVal()
	}

	if lat, lon, err := x.LatLong(); err == nil {
		meta.GPSLatitude = lat
		meta.GPSLongitude = lon
	}

	res.Image = meta
}
