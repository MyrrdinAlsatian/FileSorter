package detector

import "FileRecoveryOrganizer/types"

func DetectAssetImg(path string, size int64, img *types.ImageMeta) bool {

	if img == nil {
		return false
	}

	if img.HasExif {
		return false
	}

	if img.Width <= 128 && img.Height <= 128 {
		img.IsAsset = true
		img.Reason = "Image dimensions are typical for asset images"
		return true
	}

	if size < 10_000 {
		img.IsAsset = true
		img.Reason = "Image file size is typical for asset images"
		return true
	}

	return false
}
