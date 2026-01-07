package scanner

import "github.com/schollz/progressbar/v3"

func CreateProgessBar(total int) *progressbar.ProgressBar {
	return progressbar.Default(int64(total))
}
