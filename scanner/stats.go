package scanner

import "sync"

type SafeStats struct {
	mu sync.Mutex

	FilesByType map[string]int64
	BytesByType map[string]int64

	TotalFiles int64
	TotalSize  int64

	Errors int64
}

func NewStats() *SafeStats {
	return &SafeStats{
		FilesByType: make(map[string]int64),
		BytesByType: make(map[string]int64),
	}
}

func (s *SafeStats) AddFile(fileType string, size int64, hasError bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.FilesByType[fileType]++
	s.BytesByType[fileType] += size

	s.TotalFiles++
	s.TotalSize += size

	if hasError {
		s.Errors++
	}
}
