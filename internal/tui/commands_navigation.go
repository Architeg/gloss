package tui

func (m *Model) hasBrowseSelection() bool {
	return m.browseCursor >= 0 && m.browseCursor < len(m.cmdRows)
}

func (m *Model) entryIDExists(id int64) bool {
	if id <= 0 {
		return false
	}
	for _, entry := range m.allEntries {
		if entry.ID == id {
			return true
		}
	}
	return false
}

func (m *Model) rowIndexByID(id int64) int {
	if id <= 0 {
		return -1
	}
	for i, row := range m.cmdRows {
		if row.Entry.ID == id {
			return i
		}
	}
	return -1
}

// reconcileBrowse keeps persistent identity when possible and uses the prior
// visible position as the deterministic fallback when it is not.
func (m *Model) reconcileBrowse(fallbackIndex int) {
	if m.restoreID > 0 && !m.entryIDExists(m.restoreID) {
		m.restoreID = 0
	}
	if len(m.cmdRows) == 0 {
		if m.selectedID > 0 && m.entryIDExists(m.selectedID) && m.restoreID == 0 {
			m.restoreID = m.selectedID
		}
		m.selectedID = 0
		m.browseCursor = 0
		m.browseOffset = 0
		return
	}

	if index := m.rowIndexByID(m.restoreID); index >= 0 {
		m.browseCursor = index
		m.selectedID = m.restoreID
		m.restoreID = 0
		m.ensureBrowseVisible(true)
		return
	}
	if index := m.rowIndexByID(m.selectedID); index >= 0 {
		m.browseCursor = index
		m.ensureBrowseVisible(true)
		return
	}

	if m.selectedID > 0 && m.entryIDExists(m.selectedID) && m.restoreID == 0 {
		m.restoreID = m.selectedID
	}
	m.browseCursor = clamp(fallbackIndex, 0, len(m.cmdRows)-1)
	m.selectedID = m.cmdRows[m.browseCursor].Entry.ID
	m.ensureBrowseVisible(true)
}

func (m *Model) selectBrowseIndex(index int) {
	if len(m.cmdRows) == 0 {
		m.browseCursor = 0
		m.browseOffset = 0
		m.selectedID = 0
		return
	}
	m.browseCursor = clamp(index, 0, len(m.cmdRows)-1)
	m.selectedID = m.cmdRows[m.browseCursor].Entry.ID
	m.restoreID = 0
	m.ensureBrowseVisible(false)
}

func (m *Model) moveBrowseBy(delta int) {
	if len(m.cmdRows) == 0 {
		return
	}
	m.selectBrowseIndex(m.browseCursor + delta)
}

func (m *Model) moveBrowsePage(direction int) {
	if len(m.cmdRows) == 0 || direction == 0 {
		return
	}
	page := m.browsePageSize()
	if page == 0 {
		return
	}
	m.selectBrowseIndex(m.browseCursor + direction*page)
}

// browsePageSize is the number of selectable entries in the current viewport.
// It is at least one whenever the command-list area has at least one row.
func (m *Model) browsePageSize() int {
	fixedWidth := m.commandContentWidth()
	rowWidth := m.listRowWidth(fixedWidth)
	height := m.commandListHeight(fixedWidth)
	if height <= 0 || len(m.cmdRows) == 0 {
		return 0
	}
	start := clamp(m.browseOffset, 0, len(m.cmdRows)-1)
	_, end := m.renderCommandViewport(rowWidth, height, start)
	if end <= start {
		return 1
	}
	return end - start
}

func (m *Model) jumpBrowseGroup(direction int) {
	if !m.hasBrowseSelection() || direction == 0 {
		return
	}
	current := m.browseCursor
	start := current
	for start > 0 && samePrimaryGroup(m.cmdRows[start-1].Entry, m.cmdRows[current].Entry) {
		start--
	}

	if direction < 0 {
		if current != start {
			m.selectBrowseIndex(start)
			return
		}
		if start == 0 {
			return
		}
		previous := start - 1
		for previous > 0 && samePrimaryGroup(m.cmdRows[previous-1].Entry, m.cmdRows[previous].Entry) {
			previous--
		}
		m.selectBrowseIndex(previous)
		return
	}

	next := current + 1
	for next < len(m.cmdRows) && samePrimaryGroup(m.cmdRows[next].Entry, m.cmdRows[current].Entry) {
		next++
	}
	if next < len(m.cmdRows) {
		m.selectBrowseIndex(next)
	}
}

func (m *Model) ensureBrowseVisible(compact bool) {
	if len(m.cmdRows) == 0 {
		m.browseCursor = 0
		m.browseOffset = 0
		return
	}
	m.browseCursor = clamp(m.browseCursor, 0, len(m.cmdRows)-1)
	fixedWidth := m.commandContentWidth()
	rowWidth := m.listRowWidth(fixedWidth)
	height := m.commandListHeight(fixedWidth)
	if height <= 0 {
		m.browseOffset = 0
		return
	}
	m.browseOffset = clamp(m.browseOffset, 0, len(m.cmdRows)-1)
	if _, end := m.renderCommandViewport(rowWidth, height, 0); end == len(m.cmdRows) {
		m.browseOffset = 0
		return
	}
	if m.browseCursor < m.browseOffset {
		m.browseOffset = m.browseCursor
	}
	for {
		_, end := m.renderCommandViewport(rowWidth, height, m.browseOffset)
		if m.browseCursor < end || m.browseOffset >= m.browseCursor {
			break
		}
		m.browseOffset++
	}

	if compact {
		for m.browseOffset > 0 {
			candidate := m.browseOffset - 1
			_, end := m.renderCommandViewport(rowWidth, height, candidate)
			if m.browseCursor >= end {
				break
			}
			m.browseOffset = candidate
		}
	}

	// Pull the final page upward when possible so shrinking data cannot leave an
	// unnecessarily sparse tail.
	for m.browseOffset > 0 {
		_, end := m.renderCommandViewport(rowWidth, height, m.browseOffset)
		if end != len(m.cmdRows) {
			break
		}
		candidate := m.browseOffset - 1
		_, candidateEnd := m.renderCommandViewport(rowWidth, height, candidate)
		if candidateEnd != len(m.cmdRows) || m.browseCursor >= candidateEnd {
			break
		}
		m.browseOffset = candidate
	}
}

func (m *Model) commandContentWidth() int {
	_, _, width, _, _ := m.layoutMetrics()
	return width
}
