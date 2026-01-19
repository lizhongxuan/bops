package artifact

import "sync"

type Item struct {
	Name     string            `json:"name"`
	Type     string            `json:"type"`
	Path     string            `json:"path,omitempty"`
	Data     []byte            `json:"data,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type Collector struct {
	mu    sync.Mutex
	items []Item
}

func NewCollector() *Collector {
	return &Collector{}
}

func (c *Collector) Add(item Item) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = append(c.items, item)
}

func (c *Collector) AddLog(name string, data []byte) {
	c.Add(Item{Name: name, Type: "log", Data: data})
}

func (c *Collector) AddOutput(name string, data []byte) {
	c.Add(Item{Name: name, Type: "output", Data: data})
}

func (c *Collector) AddFile(name, path string) {
	c.Add(Item{Name: name, Type: "file", Path: path})
}

func (c *Collector) Items() []Item {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]Item, len(c.items))
	copy(out, c.items)
	return out
}
