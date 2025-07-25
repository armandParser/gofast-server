package main

import "maps"

// NewList creates a new list
func NewList() *List {
	return &List{}
}

// NewSet creates a new set
func NewSet() *Set {
	return &Set{
		members: make(map[string]struct{}),
	}
}

// NewHash creates a new hash
func NewHash() *Hash {
	return &Hash{
		fields: make(map[string][]byte),
	}
}

// List methods
func (l *List) LeftPush(value []byte) int {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	node := &ListNode{value: value}
	if l.head == nil {
		l.head = node
		l.tail = node
	} else {
		node.next = l.head
		l.head.prev = node
		l.head = node
	}
	l.length++
	return l.length
}

func (l *List) RightPush(value []byte) int {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	node := &ListNode{value: value}
	if l.tail == nil {
		l.head = node
		l.tail = node
	} else {
		l.tail.next = node
		node.prev = l.tail
		l.tail = node
	}
	l.length++
	return l.length
}

func (l *List) LeftPop() ([]byte, bool) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	if l.head == nil {
		return nil, false
	}

	value := l.head.value
	l.head = l.head.next
	if l.head != nil {
		l.head.prev = nil
	} else {
		l.tail = nil
	}
	l.length--
	return value, true
}

func (l *List) RightPop() ([]byte, bool) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	if l.tail == nil {
		return nil, false
	}

	value := l.tail.value
	l.tail = l.tail.prev
	if l.tail != nil {
		l.tail.next = nil
	} else {
		l.head = nil
	}
	l.length--
	return value, true
}

func (l *List) Length() int {
	l.mutex.RLock()
	defer l.mutex.RUnlock()
	return l.length
}

func (l *List) Index(index int) ([]byte, bool) {
	l.mutex.RLock()
	defer l.mutex.RUnlock()

	if index < 0 || index >= l.length {
		return nil, false
	}

	current := l.head
	for range index {
		current = current.next
	}
	return current.value, true
}

func (l *List) Range(start, end int) [][]byte {
	l.mutex.RLock()
	defer l.mutex.RUnlock()

	if start < 0 {
		start = 0
	}
	if end >= l.length {
		end = l.length - 1
	}
	if start > end {
		return [][]byte{}
	}

	result := make([][]byte, 0, end-start+1)
	current := l.head

	// Skip to start position
	for i := 0; i < start; i++ {
		current = current.next
	}

	// Collect values from start to end
	for i := start; i <= end && current != nil; i++ {
		result = append(result, current.value)
		current = current.next
	}

	return result
}

// Set methods
func (s *Set) Add(member string) bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	_, exists := s.members[member]
	s.members[member] = struct{}{}
	return !exists // return true if it was a new member
}

func (s *Set) Remove(member string) bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	_, exists := s.members[member]
	if exists {
		delete(s.members, member)
	}
	return exists
}

func (s *Set) Members() []string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	members := make([]string, 0, len(s.members))
	for member := range s.members {
		members = append(members, member)
	}
	return members
}

func (s *Set) Card() int {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return len(s.members)
}

func (s *Set) IsMember(member string) bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	_, exists := s.members[member]
	return exists
}

// Hash methods
func (h *Hash) Set(field string, value []byte) bool {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	_, exists := h.fields[field]
	h.fields[field] = value
	return !exists // return true if it was a new field
}

func (h *Hash) Get(field string) ([]byte, bool) {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	value, exists := h.fields[field]
	return value, exists
}

func (h *Hash) Del(field string) bool {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	_, exists := h.fields[field]
	if exists {
		delete(h.fields, field)
	}
	return exists
}

func (h *Hash) GetAll() map[string][]byte {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	result := make(map[string][]byte, len(h.fields))
	maps.Copy(result, h.fields)
	return result
}

func (h *Hash) Len() int {
	h.mutex.RLock()
	defer h.mutex.RUnlock()
	return len(h.fields)
}

func (h *Hash) Exists(field string) bool {
	h.mutex.RLock()
	defer h.mutex.RUnlock()
	_, exists := h.fields[field]
	return exists
}
