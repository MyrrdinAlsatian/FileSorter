package scanner

import (
	"FileRecoveryOrganizer/types"
	"sync"
)

type Collector struct {
	mu      sync.Mutex
	Results []types.Result
}

func NewCollector() *Collector {
	return &Collector{
		Results: make([]types.Result, 0, 1024),
	}
}

func (c *Collector) AddResult(res types.Result) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Results = append(c.Results, res)
}

func (c *Collector) GetResults() []types.Result {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]types.Result(nil), c.Results...)
}
