// SPDX-License-Identifier: MIT
package workspace

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func (m *Model) loadFiles(subpath string, depth int) {
	target := filepath.Join(m.root, subpath)
	entries, err := os.ReadDir(target)
	if err != nil {
		return
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir() != entries[j].IsDir() {
			return entries[i].IsDir()
		}
		return strings.ToLower(entries[i].Name()) < strings.ToLower(entries[j].Name())
	})

	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}

		relPath := filepath.Join(subpath, name)
		fe := fileEntry{
			name:  name,
			path:  filepath.Join(m.root, relPath),
			isDir: entry.IsDir(),
			depth: depth,
		}

		info, err := entry.Info()
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			fe.isSymlink = true
			fe.isDir = false
		}

		if fe.isDir {
			m.expanded[fe.path] = false
		}

		m.files = append(m.files, fe)
	}
}

func (m *Model) cursorUp() {
	if m.cursor > 0 {
		m.cursor--
	}
}

func (m *Model) cursorDown() {
	total := m.totalVisible()
	if m.cursor < total-1 {
		m.cursor++
	}
}

func (m *Model) toggleExpand() {
	entry := m.entryAt(m.cursor)
	if entry == nil || !entry.isDir {
		return
	}
	if m.expanded[entry.path] {
		m.collapseDir(entry)
	} else {
		m.expandDir(entry)
	}
}

func (m *Model) expandCurrent() {
	entry := m.entryAt(m.cursor)
	if entry == nil || !entry.isDir {
		return
	}
	if !m.expanded[entry.path] {
		m.expandDir(entry)
	}
}

func (m *Model) collapseCurrent() {
	entry := m.entryAt(m.cursor)
	if entry == nil || !entry.isDir {
		return
	}
	if m.expanded[entry.path] {
		m.collapseDir(entry)
	}
}

func (m *Model) expandDir(entry *fileEntry) {
	if entry.children != nil {
		m.expanded[entry.path] = true
		return
	}

	relPath, _ := filepath.Rel(m.root, entry.path)
	subFiles := m.collectFiles(relPath, entry.depth+1)
	entry.children = subFiles
	m.expanded[entry.path] = true
}

func (m *Model) collapseDir(entry *fileEntry) {
	m.expanded[entry.path] = false
	entry.children = nil
}

func (m *Model) collectFiles(subpath string, depth int) []fileEntry {
	var result []fileEntry
	target := filepath.Join(m.root, subpath)
	entries, err := os.ReadDir(target)
	if err != nil {
		return nil
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir() != entries[j].IsDir() {
			return entries[i].IsDir()
		}
		return strings.ToLower(entries[i].Name()) < strings.ToLower(entries[j].Name())
	})

	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		relPath := filepath.Join(subpath, name)
		fe := fileEntry{
			name:  name,
			path:  filepath.Join(m.root, relPath),
			isDir: entry.IsDir(),
			depth: depth,
		}

		info, err := entry.Info()
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			fe.isSymlink = true
			fe.isDir = false
		}

		result = append(result, fe)
	}
	return result
}

func (m *Model) totalVisible() int {
	count := 0
	m.countVisible(m.files, &count)
	return count
}

func (m *Model) countVisible(entries []fileEntry, count *int) {
	for _, e := range entries {
		*count++
		if e.isDir && m.expanded[e.path] {
			m.countVisible(e.children, count)
		}
	}
}

func (m *Model) entryAt(idx int) *fileEntry {
	return m.findEntry(m.files, &idx)
}

func (m *Model) findEntry(entries []fileEntry, idx *int) *fileEntry {
	for i := range entries {
		if *idx == 0 {
			return &entries[i]
		}
		*idx--
		if entries[i].isDir && m.expanded[entries[i].path] {
			if found := m.findEntry(entries[i].children, idx); found != nil {
				return found
			}
		}
	}
	return nil
}
