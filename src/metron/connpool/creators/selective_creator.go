package creators

import "io"

type Creator interface {
	Create() io.WriteCloser
}

type SelectiveCreator struct {
	creators []Creator
}

func NewSelectiveCreator(creators ...Creator) *SelectiveCreator {
	return &SelectiveCreator{
		creators: creators,
	}
}

func (c *SelectiveCreator) Create() io.WriteCloser {
	for i := 0; i < len(c.creators); i++ {
		creator := c.creators[i].Create()
		if creator == nil {
			continue
		}

		return creator
	}

	return nil
}
