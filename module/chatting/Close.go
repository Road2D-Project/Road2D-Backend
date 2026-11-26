package chatting

func (c *ChattingCoordinator) Close() { clear(c.members) }

func (p *PersonalCoordinator) Close() { clear(p.lastResync) }
