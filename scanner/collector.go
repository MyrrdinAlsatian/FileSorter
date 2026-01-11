package scanner

import (
	"FileRecoveryOrganizer/types"
	// "sync"
)

type Collector struct {
	Results chan types.Result
}

func NewCollector(buffer int) *Collector {
	return &Collector{
		Results: make(chan types.Result, buffer),
	}
}

// func (c *Collector) AddResult(res types.Result) {
// 	c.Results <- res
// 	defer c.mu.Unlock()
// 	c.Results = append(c.Results, res)
// }

// func (c *Collector) GetResults() []types.Result {
// 	c.mu.Lock()
// 	defer c.mu.Unlock()
// 	return append([]types.Result(nil), c.Results...)
// }
