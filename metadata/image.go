package metadata

import (
	"os"
	"time"

	"github.com/rwcarlsen/goexif/exif"
)

func ImageData(path string) FileData {

	f, err := os.Open(path)
	if err != nil {
		return FileData{Valid: false}
	}
	defer f.Close()

	x, err := exif.Decode(f)
	if err != nil {
		return FileData{Valid: false}
	}
	// Date Priority: DateTimeOriginal > CreateDate > ModifyDate

	tags := []string{
		"DateTimeOriginal",
		"CreateDate",
		"ModifyDate",
	}

	for _, tag := range tags {
		if t, err := x.Get(exif.FieldName(tag)); err == nil {
			dateStr, err := t.StringVal()
			if err == nil {
				// EXIF date format: "2006:01:02 15:04:05"
				const exifDateFormat = "2006:01:02 15:04:05"
				if tm, err := time.Parse(exifDateFormat, dateStr); err == nil {
					return FileData{
						Time:   tm,
						Source: "exif:" + tag,
						Valid:  true,
					}
				}
			}
		}
	}
	return FileData{Valid: false}
}
