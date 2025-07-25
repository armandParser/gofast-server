package main

import (
	"encoding/binary"
	"fmt"
	"sort"
	"strconv"
)

func (s *GoFastServer) handleMGet(data []byte, now int64) []byte {
	// Parse multiple keys from data: [count:4][key1_len:4][key1][key2_len:4][key2]...
	if len(data) < 4 {
		return s.createResponse(RESP_ERROR, []byte("Invalid MGET data"))
	}

	count := binary.BigEndian.Uint32(data[0:4])
	if count == 0 {
		return s.createResponse(RESP_OK, s.encodeMGetResponse([][]byte{}))
	}

	keys := make([]string, count)
	offset := 4

	// Parse all keys
	for i := range count {
		if offset+4 > len(data) {
			return s.createResponse(RESP_ERROR, []byte("Invalid MGET data - insufficient data"))
		}

		keyLen := binary.BigEndian.Uint32(data[offset : offset+4])
		offset += 4

		if offset+int(keyLen) > len(data) {
			return s.createResponse(RESP_ERROR, []byte("Invalid MGET data - key too long"))
		}

		keys[i] = string(data[offset : offset+int(keyLen)])
		offset += int(keyLen)
	}

	// Fetch all values
	values := make([][]byte, count)
	for i, key := range keys {
		if value, exists := s.storage.Load(key); exists {
			item := value.(*CacheItem)

			// Check if expired
			if item.ExpiresAt > 0 && item.ExpiresAt <= now {
				s.storage.Delete(key)
				s.ttlMutex.Lock()
				delete(s.ttlIndex, key)
				s.ttlMutex.Unlock()
				values[i] = nil // Expired/not found
			} else if item.DataType == TYPE_STRING {
				values[i] = item.Value.([]byte)
			} else {
				values[i] = nil // Wrong type
			}
		} else {
			values[i] = nil // Not found
		}
	}

	return s.createResponse(RESP_OK, s.encodeMGetResponse(values))
}

// STEP 4: Add the MSET handler to main.go (add after handleMGet function)

func (s *GoFastServer) handleMSet(data []byte, now int64) []byte {
	// Parse multiple key-value pairs: [count:4][key1_len:4][key1][val1_len:4][val1][ttl1:4]...
	if len(data) < 4 {
		return s.createResponse(RESP_ERROR, []byte("Invalid MSET data"))
	}

	count := binary.BigEndian.Uint32(data[0:4])
	if count == 0 {
		return s.createResponse(RESP_OK, []byte("0"))
	}

	offset := 4
	successCount := 0

	// Parse and set all key-value pairs
	for range count {
		// Parse key
		if offset+4 > len(data) {
			return s.createResponse(RESP_ERROR, []byte("Invalid MSET data - insufficient key length"))
		}

		keyLen := binary.BigEndian.Uint32(data[offset : offset+4])
		offset += 4

		if offset+int(keyLen) > len(data) {
			return s.createResponse(RESP_ERROR, []byte("Invalid MSET data - key too long"))
		}

		key := string(data[offset : offset+int(keyLen)])
		offset += int(keyLen)

		// Parse value
		if offset+4 > len(data) {
			return s.createResponse(RESP_ERROR, []byte("Invalid MSET data - insufficient value length"))
		}

		valueLen := binary.BigEndian.Uint32(data[offset : offset+4])
		offset += 4

		if offset+int(valueLen) > len(data) {
			return s.createResponse(RESP_ERROR, []byte("Invalid MSET data - value too long"))
		}

		value := make([]byte, valueLen)
		copy(value, data[offset:offset+int(valueLen)])
		offset += int(valueLen)

		// Parse TTL
		if offset+4 > len(data) {
			return s.createResponse(RESP_ERROR, []byte("Invalid MSET data - insufficient TTL"))
		}

		ttl := binary.BigEndian.Uint32(data[offset : offset+4])
		offset += 4

		// Store the key-value pair
		item := &CacheItem{
			DataType:  TYPE_STRING,
			Value:     value,
			CreatedAt: now,
		}

		if ttl > 0 {
			item.ExpiresAt = now + int64(ttl)
			s.ttlMutex.Lock()
			s.ttlIndex[key] = item.ExpiresAt
			s.ttlMutex.Unlock()
		}

		s.storage.Store(key, item)
		successCount++
	}

	return s.createResponse(RESP_OK, []byte(fmt.Sprintf("%d", successCount)))
}

func (s *GoFastServer) handlePipeline(data []byte, now int64) []byte {
	// Parse pipeline: [count:4][msg1][msg2][msg3]...
	if len(data) < 4 {
		return s.createResponse(RESP_ERROR, []byte("Invalid PIPELINE data"))
	}

	count := binary.BigEndian.Uint32(data[0:4])
	if count == 0 {
		return s.createResponse(RESP_OK, s.encodePipelineResponse([][]byte{}))
	}

	responses := make([][]byte, count)
	offset := 4

	// Process each command in the pipeline
	for i := range count {
		if offset >= len(data) {
			responses[i] = s.createResponse(RESP_ERROR, []byte("Incomplete pipeline command"))
			continue
		}

		// Parse individual message from pipeline data
		msg, newOffset, err := s.parsePipelineMessage(data, offset)
		if err != nil {
			responses[i] = s.createResponse(RESP_ERROR, []byte(fmt.Sprintf("Pipeline parse error: %v", err)))
			offset = newOffset
			continue
		}

		// Process the individual command
		response := s.processIndividualCommand(msg, now)
		responses[i] = response
		offset = newOffset
	}

	return s.createResponse(RESP_OK, s.encodePipelineResponse(responses))
}

//  New parsePipelineMessage() function (add after handlePipeline()):

func (s *GoFastServer) parsePipelineMessage(data []byte, offset int) (*Message, int, error) {
	if offset+6 > len(data) { // minimum: length(4) + version(1) + command(1)
		return nil, offset, fmt.Errorf("insufficient data for message header")
	}

	// Read message length
	msgLen := binary.BigEndian.Uint32(data[offset : offset+4])
	offset += 4

	if offset+int(msgLen) > len(data) {
		return nil, offset, fmt.Errorf("message length exceeds available data")
	}

	// Read version and command
	version := data[offset]
	command := data[offset+1]
	offset += 2

	msg := &Message{
		Length:  msgLen,
		Version: version,
		Command: command,
	}

	// Parse based on command type (reuse existing parsing logic)
	remaining := int(msgLen) - 2 // subtract version and command bytes
	endOffset := offset + remaining

	switch command {
	case CMD_SET:
		if remaining < 12 {
			return nil, endOffset, fmt.Errorf("invalid SET message in pipeline")
		}
		// Parse SET: [keylen:4][key][ttl:4][valuelen:4][value]
		keyLen := binary.BigEndian.Uint32(data[offset : offset+4])
		offset += 4
		msg.Key = make([]byte, keyLen)
		copy(msg.Key, data[offset:offset+int(keyLen)])
		offset += int(keyLen)

		msg.TTL = binary.BigEndian.Uint32(data[offset : offset+4])
		offset += 4

		valueLen := binary.BigEndian.Uint32(data[offset : offset+4])
		offset += 4
		msg.Value = make([]byte, valueLen)
		copy(msg.Value, data[offset:offset+int(valueLen)])

	case CMD_EXPIRE:
		// Parse EXPIRE: [keylen:4][key][ttl:4]
		if remaining < 8 {
			return nil, endOffset, fmt.Errorf("invalid EXPIRE message in pipeline")
		}
		keyLen := binary.BigEndian.Uint32(data[offset : offset+4])
		offset += 4
		msg.Key = make([]byte, keyLen)
		copy(msg.Key, data[offset:offset+int(keyLen)])
		offset += int(keyLen)
		msg.TTL = binary.BigEndian.Uint32(data[offset : offset+4])

	case CMD_LPUSH, CMD_RPUSH, CMD_SADD, CMD_GETSET:
		// Parse list/set/getset operations: [keylen:4][key][valuelen:4][value]
		if remaining < 8 {
			return nil, endOffset, fmt.Errorf("invalid list/set operation in pipeline")
		}
		keyLen := binary.BigEndian.Uint32(data[offset : offset+4])
		offset += 4
		msg.Key = make([]byte, keyLen)
		copy(msg.Key, data[offset:offset+int(keyLen)])
		offset += int(keyLen)

		valueLen := binary.BigEndian.Uint32(data[offset : offset+4])
		offset += 4
		msg.Value = make([]byte, valueLen)
		copy(msg.Value, data[offset:offset+int(valueLen)])

	case CMD_SCAN:
		// Parse SCAN: [cursor:4][patternlen:4][pattern]
		if remaining < 8 {
			return nil, endOffset, fmt.Errorf("invalid SCAN message in pipeline")
		}
		msg.TTL = binary.BigEndian.Uint32(data[offset : offset+4]) // cursor stored in TTL field
		offset += 4

		patternLen := binary.BigEndian.Uint32(data[offset : offset+4])
		offset += 4
		msg.Value = make([]byte, patternLen)
		copy(msg.Value, data[offset:offset+int(patternLen)])
		offset += int(patternLen)

	case CMD_HSET:
		// Parse HSET: [keylen:4][key][fieldlen:4][field][valuelen:4][value]
		if remaining < 12 {
			return nil, endOffset, fmt.Errorf("invalid HSET message in pipeline")
		}
		keyLen := binary.BigEndian.Uint32(data[offset : offset+4])
		offset += 4
		msg.Key = make([]byte, keyLen)
		copy(msg.Key, data[offset:offset+int(keyLen)])
		offset += int(keyLen)

		fieldLen := binary.BigEndian.Uint32(data[offset : offset+4])
		offset += 4
		fieldBytes := make([]byte, fieldLen)
		copy(fieldBytes, data[offset:offset+int(fieldLen)])
		offset += int(fieldLen)

		valueLen := binary.BigEndian.Uint32(data[offset : offset+4])
		offset += 4

		// Pack field and value together like in original HSET parsing
		msg.Value = make([]byte, 4+fieldLen+valueLen)
		binary.BigEndian.PutUint32(msg.Value[0:4], fieldLen)
		copy(msg.Value[4:], fieldBytes)
		copy(msg.Value[4+fieldLen:], data[offset:offset+int(valueLen)])

	case CMD_HGET, CMD_HDEL, CMD_HEXISTS:
		// Parse hash field operations: [keylen:4][key][fieldlen:4][field]
		if remaining < 8 {
			return nil, endOffset, fmt.Errorf("invalid hash field operation in pipeline")
		}
		keyLen := binary.BigEndian.Uint32(data[offset : offset+4])
		offset += 4
		msg.Key = make([]byte, keyLen)
		copy(msg.Key, data[offset:offset+int(keyLen)])
		offset += int(keyLen)

		fieldLen := binary.BigEndian.Uint32(data[offset : offset+4])
		offset += 4
		msg.Value = make([]byte, fieldLen)
		copy(msg.Value, data[offset:offset+int(fieldLen)])

	case CMD_LINDEX:
		// Parse LINDEX: [keylen:4][key][index:4]
		if remaining < 8 {
			return nil, endOffset, fmt.Errorf("invalid LINDEX message in pipeline")
		}
		keyLen := binary.BigEndian.Uint32(data[offset : offset+4])
		offset += 4
		msg.Key = make([]byte, keyLen)
		copy(msg.Key, data[offset:offset+int(keyLen)])
		offset += int(keyLen)
		msg.TTL = binary.BigEndian.Uint32(data[offset : offset+4]) // Reusing TTL field for index

	case CMD_LRANGE:
		// Parse LRANGE: [keylen:4][key][start:4][end:4]
		if remaining < 12 {
			return nil, endOffset, fmt.Errorf("invalid LRANGE message in pipeline")
		}
		keyLen := binary.BigEndian.Uint32(data[offset : offset+4])
		offset += 4
		msg.Key = make([]byte, keyLen)
		copy(msg.Key, data[offset:offset+int(keyLen)])
		offset += int(keyLen)

		msg.TTL = binary.BigEndian.Uint32(data[offset : offset+4]) // start
		offset += 4

		// Store end in first 4 bytes of Value
		msg.Value = make([]byte, 4)
		endBytes := data[offset : offset+4]
		copy(msg.Value, endBytes)

	case CMD_LPOP, CMD_RPOP, CMD_SREM, CMD_SISMEMBER:
		// These were already handled in the original code, but let's be explicit
		keyLen := binary.BigEndian.Uint32(data[offset : offset+4])
		offset += 4
		msg.Key = make([]byte, keyLen)
		copy(msg.Key, data[offset:offset+int(keyLen)])
		offset += int(keyLen)

		// For SREM and SISMEMBER, read value if present
		remainingAfterKey := remaining - 4 - int(keyLen)
		if remainingAfterKey > 0 && (msg.Command == CMD_SREM || msg.Command == CMD_SISMEMBER) {
			valueLenBytes := data[offset : offset+4]
			valueLen := binary.BigEndian.Uint32(valueLenBytes)
			offset += 4
			msg.Value = make([]byte, valueLen)
			copy(msg.Value, data[offset:offset+int(valueLen)])
		}

	case CMD_GET, CMD_DEL, CMD_EXISTS, CMD_TTL, CMD_LLEN, CMD_SMEMBERS, CMD_SCARD, CMD_HGETALL, CMD_HLEN, CMD_INCR, CMD_DECR, CMD_KEYS:
		// Parse simple key-only commands: [keylen:4][key]
		if remaining < 4 {
			return nil, endOffset, fmt.Errorf("invalid key-only message in pipeline")
		}
		keyLen := binary.BigEndian.Uint32(data[offset : offset+4])
		offset += 4
		msg.Key = make([]byte, keyLen)
		copy(msg.Key, data[offset:offset+int(keyLen)])

	default:
		return nil, endOffset, fmt.Errorf("unsupported command in pipeline: %d", command)
	}

	return msg, endOffset, nil
}

// List operation handlers
func (s *GoFastServer) handleListPush(key string, value []byte, isLeft bool, now int64) []byte {
	var list *List

	if existing, exists := s.storage.Load(key); exists {
		item := existing.(*CacheItem)
		if item.ExpiresAt > 0 && item.ExpiresAt <= now {
			s.storage.Delete(key)
			s.ttlMutex.Lock()
			delete(s.ttlIndex, key)
			s.ttlMutex.Unlock()
		} else if item.DataType != TYPE_LIST {
			return s.createResponse(RESP_ERROR, []byte("WRONGTYPE Operation against a key holding the wrong kind of value"))
		} else {
			list = item.Value.(*List)
		}
	}

	if list == nil {
		list = NewList()
		item := &CacheItem{
			DataType:  TYPE_LIST,
			Value:     list,
			CreatedAt: now,
		}
		s.storage.Store(key, item)
	}

	var length int
	if isLeft {
		length = list.LeftPush(value)
	} else {
		length = list.RightPush(value)
	}

	return s.createResponse(RESP_OK, []byte(fmt.Sprintf("%d", length)))
}

func (s *GoFastServer) handleListPop(key string, isLeft bool, now int64) []byte {
	existing, exists := s.storage.Load(key)
	if !exists {
		return s.createResponse(RESP_NOT_FOUND, nil)
	}

	item := existing.(*CacheItem)
	if item.ExpiresAt > 0 && item.ExpiresAt <= now {
		s.storage.Delete(key)
		s.ttlMutex.Lock()
		delete(s.ttlIndex, key)
		s.ttlMutex.Unlock()
		return s.createResponse(RESP_NOT_FOUND, nil)
	}

	if item.DataType != TYPE_LIST {
		return s.createResponse(RESP_ERROR, []byte("WRONGTYPE Operation against a key holding the wrong kind of value"))
	}

	list := item.Value.(*List)
	var value []byte
	var ok bool

	if isLeft {
		value, ok = list.LeftPop()
	} else {
		value, ok = list.RightPop()
	}

	if !ok {
		return s.createResponse(RESP_NOT_FOUND, nil)
	}

	// If list is now empty, remove the key
	if list.Length() == 0 {
		s.storage.Delete(key)
		s.ttlMutex.Lock()
		delete(s.ttlIndex, key)
		s.ttlMutex.Unlock()
	}

	return s.createResponse(RESP_OK, value)
}

func (s *GoFastServer) handleListLen(key string, now int64) []byte {
	existing, exists := s.storage.Load(key)
	if !exists {
		return s.createResponse(RESP_OK, []byte("0"))
	}

	item := existing.(*CacheItem)
	if item.ExpiresAt > 0 && item.ExpiresAt <= now {
		s.storage.Delete(key)
		s.ttlMutex.Lock()
		delete(s.ttlIndex, key)
		s.ttlMutex.Unlock()
		return s.createResponse(RESP_OK, []byte("0"))
	}

	if item.DataType != TYPE_LIST {
		return s.createResponse(RESP_ERROR, []byte("WRONGTYPE Operation against a key holding the wrong kind of value"))
	}

	list := item.Value.(*List)
	return s.createResponse(RESP_OK, []byte(fmt.Sprintf("%d", list.Length())))
}

func (s *GoFastServer) handleListIndex(key string, index int, now int64) []byte {
	existing, exists := s.storage.Load(key)
	if !exists {
		return s.createResponse(RESP_NOT_FOUND, nil)
	}

	item := existing.(*CacheItem)
	if item.ExpiresAt > 0 && item.ExpiresAt <= now {
		s.storage.Delete(key)
		s.ttlMutex.Lock()
		delete(s.ttlIndex, key)
		s.ttlMutex.Unlock()
		return s.createResponse(RESP_NOT_FOUND, nil)
	}

	if item.DataType != TYPE_LIST {
		return s.createResponse(RESP_ERROR, []byte("WRONGTYPE Operation against a key holding the wrong kind of value"))
	}

	list := item.Value.(*List)
	value, ok := list.Index(index)
	if !ok {
		return s.createResponse(RESP_NOT_FOUND, nil)
	}

	return s.createResponse(RESP_OK, value)
}

func (s *GoFastServer) handleListRange(key string, start, end int, now int64) []byte {
	existing, exists := s.storage.Load(key)
	if !exists {
		return s.createResponse(RESP_OK, s.encodeArray([][]byte{}))
	}

	item := existing.(*CacheItem)
	if item.ExpiresAt > 0 && item.ExpiresAt <= now {
		s.storage.Delete(key)
		s.ttlMutex.Lock()
		delete(s.ttlIndex, key)
		s.ttlMutex.Unlock()
		return s.createResponse(RESP_OK, s.encodeArray([][]byte{}))
	}

	if item.DataType != TYPE_LIST {
		return s.createResponse(RESP_ERROR, []byte("WRONGTYPE Operation against a key holding the wrong kind of value"))
	}

	list := item.Value.(*List)
	values := list.Range(start, end)

	return s.createResponse(RESP_OK, s.encodeArray(values))
}

// Set operation handlers
func (s *GoFastServer) handleSetAdd(key string, member string, now int64) []byte {
	var set *Set

	if existing, exists := s.storage.Load(key); exists {
		item := existing.(*CacheItem)
		if item.ExpiresAt > 0 && item.ExpiresAt <= now {
			s.storage.Delete(key)
			s.ttlMutex.Lock()
			delete(s.ttlIndex, key)
			s.ttlMutex.Unlock()
		} else if item.DataType != TYPE_SET {
			return s.createResponse(RESP_ERROR, []byte("WRONGTYPE Operation against a key holding the wrong kind of value"))
		} else {
			set = item.Value.(*Set)
		}
	}

	if set == nil {
		set = NewSet()
		item := &CacheItem{
			DataType:  TYPE_SET,
			Value:     set,
			CreatedAt: now,
		}
		s.storage.Store(key, item)
	}

	wasNew := set.Add(member)
	if wasNew {
		return s.createResponse(RESP_OK, []byte("1"))
	}
	return s.createResponse(RESP_OK, []byte("0"))
}

func (s *GoFastServer) handleSetRem(key string, member string, now int64) []byte {
	existing, exists := s.storage.Load(key)
	if !exists {
		return s.createResponse(RESP_OK, []byte("0"))
	}

	item := existing.(*CacheItem)
	if item.ExpiresAt > 0 && item.ExpiresAt <= now {
		s.storage.Delete(key)
		s.ttlMutex.Lock()
		delete(s.ttlIndex, key)
		s.ttlMutex.Unlock()
		return s.createResponse(RESP_OK, []byte("0"))
	}

	if item.DataType != TYPE_SET {
		return s.createResponse(RESP_ERROR, []byte("WRONGTYPE Operation against a key holding the wrong kind of value"))
	}

	set := item.Value.(*Set)
	removed := set.Remove(member)

	// If set is now empty, remove the key
	if set.Card() == 0 {
		s.storage.Delete(key)
		s.ttlMutex.Lock()
		delete(s.ttlIndex, key)
		s.ttlMutex.Unlock()
	}

	if removed {
		return s.createResponse(RESP_OK, []byte("1"))
	}
	return s.createResponse(RESP_OK, []byte("0"))
}

func (s *GoFastServer) handleSetMembers(key string, now int64) []byte {
	existing, exists := s.storage.Load(key)
	if !exists {
		return s.createResponse(RESP_OK, s.encodeStringArray([]string{}))
	}

	item := existing.(*CacheItem)
	if item.ExpiresAt > 0 && item.ExpiresAt <= now {
		s.storage.Delete(key)
		s.ttlMutex.Lock()
		delete(s.ttlIndex, key)
		s.ttlMutex.Unlock()
		return s.createResponse(RESP_OK, s.encodeStringArray([]string{}))
	}

	if item.DataType != TYPE_SET {
		return s.createResponse(RESP_ERROR, []byte("WRONGTYPE Operation against a key holding the wrong kind of value"))
	}

	set := item.Value.(*Set)
	members := set.Members()

	return s.createResponse(RESP_OK, s.encodeStringArray(members))
}

func (s *GoFastServer) handleSetCard(key string, now int64) []byte {
	existing, exists := s.storage.Load(key)
	if !exists {
		return s.createResponse(RESP_OK, []byte("0"))
	}

	item := existing.(*CacheItem)
	if item.ExpiresAt > 0 && item.ExpiresAt <= now {
		s.storage.Delete(key)
		s.ttlMutex.Lock()
		delete(s.ttlIndex, key)
		s.ttlMutex.Unlock()
		return s.createResponse(RESP_OK, []byte("0"))
	}

	if item.DataType != TYPE_SET {
		return s.createResponse(RESP_ERROR, []byte("WRONGTYPE Operation against a key holding the wrong kind of value"))
	}

	set := item.Value.(*Set)
	return s.createResponse(RESP_OK, []byte(fmt.Sprintf("%d", set.Card())))
}

func (s *GoFastServer) handleSetIsMember(key string, member string, now int64) []byte {
	existing, exists := s.storage.Load(key)
	if !exists {
		return s.createResponse(RESP_OK, []byte("0"))
	}

	item := existing.(*CacheItem)
	if item.ExpiresAt > 0 && item.ExpiresAt <= now {
		s.storage.Delete(key)
		s.ttlMutex.Lock()
		delete(s.ttlIndex, key)
		s.ttlMutex.Unlock()
		return s.createResponse(RESP_OK, []byte("0"))
	}

	if item.DataType != TYPE_SET {
		return s.createResponse(RESP_ERROR, []byte("WRONGTYPE Operation against a key holding the wrong kind of value"))
	}

	set := item.Value.(*Set)
	if set.IsMember(member) {
		return s.createResponse(RESP_OK, []byte("1"))
	}
	return s.createResponse(RESP_OK, []byte("0"))
}

// Hash operation handlers
func (s *GoFastServer) handleHashSet(key string, data []byte, now int64) []byte {
	// Parse field and value from data: [fieldlen:4][field][value]
	if len(data) < 4 {
		return s.createResponse(RESP_ERROR, []byte("Invalid HSET data"))
	}

	fieldLen := binary.BigEndian.Uint32(data[0:4])
	if len(data) < int(4+fieldLen) {
		return s.createResponse(RESP_ERROR, []byte("Invalid HSET data"))
	}

	field := string(data[4 : 4+fieldLen])
	value := data[4+fieldLen:]

	var hash *Hash

	if existing, exists := s.storage.Load(key); exists {
		item := existing.(*CacheItem)
		if item.ExpiresAt > 0 && item.ExpiresAt <= now {
			s.storage.Delete(key)
			s.ttlMutex.Lock()
			delete(s.ttlIndex, key)
			s.ttlMutex.Unlock()
		} else if item.DataType != TYPE_HASH {
			return s.createResponse(RESP_ERROR, []byte("WRONGTYPE Operation against a key holding the wrong kind of value"))
		} else {
			hash = item.Value.(*Hash)
		}
	}

	if hash == nil {
		hash = NewHash()
		item := &CacheItem{
			DataType:  TYPE_HASH,
			Value:     hash,
			CreatedAt: now,
		}
		s.storage.Store(key, item)
	}

	wasNew := hash.Set(field, value)
	if wasNew {
		return s.createResponse(RESP_OK, []byte("1"))
	}
	return s.createResponse(RESP_OK, []byte("0"))
}

func (s *GoFastServer) handleHashGet(key string, field string, now int64) []byte {
	existing, exists := s.storage.Load(key)
	if !exists {
		return s.createResponse(RESP_NOT_FOUND, nil)
	}

	item := existing.(*CacheItem)
	if item.ExpiresAt > 0 && item.ExpiresAt <= now {
		s.storage.Delete(key)
		s.ttlMutex.Lock()
		delete(s.ttlIndex, key)
		s.ttlMutex.Unlock()
		return s.createResponse(RESP_NOT_FOUND, nil)
	}

	if item.DataType != TYPE_HASH {
		return s.createResponse(RESP_ERROR, []byte("WRONGTYPE Operation against a key holding the wrong kind of value"))
	}

	hash := item.Value.(*Hash)
	value, exists := hash.Get(field)
	if !exists {
		return s.createResponse(RESP_NOT_FOUND, nil)
	}

	return s.createResponse(RESP_OK, value)
}

func (s *GoFastServer) handleHashDel(key string, field string, now int64) []byte {
	existing, exists := s.storage.Load(key)
	if !exists {
		return s.createResponse(RESP_OK, []byte("0"))
	}

	item := existing.(*CacheItem)
	if item.ExpiresAt > 0 && item.ExpiresAt <= now {
		s.storage.Delete(key)
		s.ttlMutex.Lock()
		delete(s.ttlIndex, key)
		s.ttlMutex.Unlock()
		return s.createResponse(RESP_OK, []byte("0"))
	}

	if item.DataType != TYPE_HASH {
		return s.createResponse(RESP_ERROR, []byte("WRONGTYPE Operation against a key holding the wrong kind of value"))
	}

	hash := item.Value.(*Hash)
	removed := hash.Del(field)

	// If hash is now empty, remove the key
	if hash.Len() == 0 {
		s.storage.Delete(key)
		s.ttlMutex.Lock()
		delete(s.ttlIndex, key)
		s.ttlMutex.Unlock()
	}

	if removed {
		return s.createResponse(RESP_OK, []byte("1"))
	}
	return s.createResponse(RESP_OK, []byte("0"))
}

func (s *GoFastServer) handleHashGetAll(key string, now int64) []byte {
	existing, exists := s.storage.Load(key)
	if !exists {
		return s.createResponse(RESP_OK, s.encodeHashMap(map[string][]byte{}))
	}

	item := existing.(*CacheItem)
	if item.ExpiresAt > 0 && item.ExpiresAt <= now {
		s.storage.Delete(key)
		s.ttlMutex.Lock()
		delete(s.ttlIndex, key)
		s.ttlMutex.Unlock()
		return s.createResponse(RESP_OK, s.encodeHashMap(map[string][]byte{}))
	}

	if item.DataType != TYPE_HASH {
		return s.createResponse(RESP_ERROR, []byte("WRONGTYPE Operation against a key holding the wrong kind of value"))
	}

	hash := item.Value.(*Hash)
	fields := hash.GetAll()

	return s.createResponse(RESP_OK, s.encodeHashMap(fields))
}

func (s *GoFastServer) handleHashLen(key string, now int64) []byte {
	existing, exists := s.storage.Load(key)
	if !exists {
		return s.createResponse(RESP_OK, []byte("0"))
	}

	item := existing.(*CacheItem)
	if item.ExpiresAt > 0 && item.ExpiresAt <= now {
		s.storage.Delete(key)
		s.ttlMutex.Lock()
		delete(s.ttlIndex, key)
		s.ttlMutex.Unlock()
		return s.createResponse(RESP_OK, []byte("0"))
	}

	if item.DataType != TYPE_HASH {
		return s.createResponse(RESP_ERROR, []byte("WRONGTYPE Operation against a key holding the wrong kind of value"))
	}

	hash := item.Value.(*Hash)
	return s.createResponse(RESP_OK, []byte(fmt.Sprintf("%d", hash.Len())))
}

func (s *GoFastServer) handleHashExists(key string, field string, now int64) []byte {
	existing, exists := s.storage.Load(key)
	if !exists {
		return s.createResponse(RESP_OK, []byte("0"))
	}

	item := existing.(*CacheItem)
	if item.ExpiresAt > 0 && item.ExpiresAt <= now {
		s.storage.Delete(key)
		s.ttlMutex.Lock()
		delete(s.ttlIndex, key)
		s.ttlMutex.Unlock()
		return s.createResponse(RESP_OK, []byte("0"))
	}

	if item.DataType != TYPE_HASH {
		return s.createResponse(RESP_ERROR, []byte("WRONGTYPE Operation against a key holding the wrong kind of value"))
	}

	hash := item.Value.(*Hash)
	if hash.Exists(field) {
		return s.createResponse(RESP_OK, []byte("1"))
	}
	return s.createResponse(RESP_OK, []byte("0"))
}

// Add to handlers.go

func (s *GoFastServer) handleIncr(key string, now int64) []byte {
	existing, exists := s.storage.Load(key)

	var currentValue int64 = 0

	if exists {
		item := existing.(*CacheItem)

		// Check if expired
		if item.ExpiresAt > 0 && item.ExpiresAt <= now {
			s.storage.Delete(key)
			s.ttlMutex.Lock()
			delete(s.ttlIndex, key)
			s.ttlMutex.Unlock()
			// Will create new key with value 1
		} else if item.DataType != TYPE_STRING {
			return s.createResponse(RESP_ERROR, []byte("WRONGTYPE Operation against a key holding the wrong kind of value"))
		} else {
			// Parse current value
			valueStr := string(item.Value.([]byte))
			if parsed, err := strconv.ParseInt(valueStr, 10, 64); err != nil {
				return s.createResponse(RESP_ERROR, []byte("ERR value is not an integer or out of range"))
			} else {
				currentValue = parsed
			}
		}
	}

	// Increment
	newValue := currentValue + 1
	newValueStr := strconv.FormatInt(newValue, 10)

	// Store the new value
	item := &CacheItem{
		DataType:  TYPE_STRING,
		Value:     []byte(newValueStr),
		CreatedAt: now,
	}

	// Preserve TTL if it existed
	if exists {
		if existingItem := existing.(*CacheItem); existingItem.ExpiresAt > 0 {
			item.ExpiresAt = existingItem.ExpiresAt
		}
	}

	s.storage.Store(key, item)
	return s.createResponse(RESP_OK, []byte(newValueStr))
}

func (s *GoFastServer) handleDecr(key string, now int64) []byte {
	existing, exists := s.storage.Load(key)

	var currentValue int64 = 0

	if exists {
		item := existing.(*CacheItem)

		// Check if expired
		if item.ExpiresAt > 0 && item.ExpiresAt <= now {
			s.storage.Delete(key)
			s.ttlMutex.Lock()
			delete(s.ttlIndex, key)
			s.ttlMutex.Unlock()
			// Will create new key with value -1
		} else if item.DataType != TYPE_STRING {
			return s.createResponse(RESP_ERROR, []byte("WRONGTYPE Operation against a key holding the wrong kind of value"))
		} else {
			// Parse current value
			valueStr := string(item.Value.([]byte))
			if parsed, err := strconv.ParseInt(valueStr, 10, 64); err != nil {
				return s.createResponse(RESP_ERROR, []byte("ERR value is not an integer or out of range"))
			} else {
				currentValue = parsed
			}
		}
	}

	// Decrement
	newValue := currentValue - 1
	newValueStr := strconv.FormatInt(newValue, 10)

	// Store the new value
	item := &CacheItem{
		DataType:  TYPE_STRING,
		Value:     []byte(newValueStr),
		CreatedAt: now,
	}

	// Preserve TTL if it existed
	if exists {
		if existingItem := existing.(*CacheItem); existingItem.ExpiresAt > 0 {
			item.ExpiresAt = existingItem.ExpiresAt
		}
	}

	s.storage.Store(key, item)
	return s.createResponse(RESP_OK, []byte(newValueStr))
}

func (s *GoFastServer) handleGetSet(key string, newValue []byte, now int64) []byte {
	existing, exists := s.storage.Load(key)

	var oldValue []byte
	var preserveTTL int64 = 0

	if exists {
		item := existing.(*CacheItem)

		// Check if expired
		if item.ExpiresAt > 0 && item.ExpiresAt <= now {
			s.storage.Delete(key)
			s.ttlMutex.Lock()
			delete(s.ttlIndex, key)
			s.ttlMutex.Unlock()
			// Treat as if key didn't exist
		} else if item.DataType != TYPE_STRING {
			return s.createResponse(RESP_ERROR, []byte("WRONGTYPE Operation against a key holding the wrong kind of value"))
		} else {
			oldValue = item.Value.([]byte)
			preserveTTL = item.ExpiresAt
		}
	}

	// Set the new value
	item := &CacheItem{
		DataType:  TYPE_STRING,
		Value:     newValue,
		CreatedAt: now,
		ExpiresAt: preserveTTL, // Preserve existing TTL
	}

	s.storage.Store(key, item)

	// Return old value or nil if key didn't exist
	if oldValue != nil {
		return s.createResponse(RESP_OK, oldValue)
	}
	return s.createResponse(RESP_NOT_FOUND, nil)
}

// Add to handlers.go

func (s *GoFastServer) handleKeys(pattern string, now int64) []byte {
	var matchingKeys []string

	// Iterate through all keys in storage
	s.storage.Range(func(key, value any) bool {
		keyStr := key.(string)
		item := value.(*CacheItem)

		// Check if key is expired
		if item.ExpiresAt > 0 && item.ExpiresAt <= now {
			// Mark for deletion (we'll clean up later)
			go func(k string) {
				s.storage.Delete(k)
				s.ttlMutex.Lock()
				delete(s.ttlIndex, k)
				s.ttlMutex.Unlock()
			}(keyStr)
			return true // Continue iteration
		}

		// Check if key matches pattern
		if s.matchPattern(pattern, keyStr) {
			matchingKeys = append(matchingKeys, keyStr)
		}

		return true // Continue iteration
	})

	return s.createResponse(RESP_OK, s.encodeStringArray(matchingKeys))
}

func (s *GoFastServer) handleScan(cursor uint32, pattern string, count int, now int64) []byte {
	var matchingKeys []string
	var keys []string
	nextCursor := uint32(0)

	// First, collect all non-expired keys
	s.storage.Range(func(key, value any) bool {
		keyStr := key.(string)
		item := value.(*CacheItem)

		// Check if key is expired
		if item.ExpiresAt > 0 && item.ExpiresAt <= now {
			// Mark for deletion
			go func(k string) {
				s.storage.Delete(k)
				s.ttlMutex.Lock()
				delete(s.ttlIndex, k)
				s.ttlMutex.Unlock()
			}(keyStr)
			return true
		}

		keys = append(keys, keyStr)
		return true
	})

	// Sort keys for consistent iteration
	sort.Strings(keys)

	// Apply cursor-based pagination
	startIndex := int(cursor)
	if startIndex >= len(keys) {
		// Cursor is beyond available keys, return empty result
		return s.createResponse(RESP_OK, s.encodeScanResponse(0, []string{}))
	}

	// Collect up to 'count' keys starting from cursor position
	endIndex := startIndex + count
	if endIndex > len(keys) {
		endIndex = len(keys)
		nextCursor = 0 // No more keys
	} else {
		nextCursor = uint32(endIndex)
	}

	// Filter by pattern
	for i := startIndex; i < endIndex; i++ {
		if s.matchPattern(pattern, keys[i]) {
			matchingKeys = append(matchingKeys, keys[i])
		}
	}

	return s.createResponse(RESP_OK, s.encodeScanResponse(nextCursor, matchingKeys))
}

// Helper function for pattern matching (supports * and ? wildcards)
func (s *GoFastServer) matchPattern(pattern, key string) bool {
	// If no pattern specified, match all
	if pattern == "" || pattern == "*" {
		return true
	}

	// Simple pattern matching implementation
	return s.wildcardMatch(pattern, key)
}

// Wildcard matching function
func (s *GoFastServer) wildcardMatch(pattern, str string) bool {
	i, j := 0, 0
	starIdx, match := -1, 0

	for i < len(str) {
		if j < len(pattern) && (pattern[j] == '?' || pattern[j] == str[i]) {
			i++
			j++
		} else if j < len(pattern) && pattern[j] == '*' {
			starIdx = j
			match = i
			j++
		} else if starIdx != -1 {
			j = starIdx + 1
			match++
			i = match
		} else {
			return false
		}
	}

	// Skip any remaining '*' in pattern
	for j < len(pattern) && pattern[j] == '*' {
		j++
	}

	return j == len(pattern)
}
