package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"time"
)

// readMessage reads a binary message from the connection
func (s *GoFastServer) readMessage(reader *bufio.Reader) (*Message, error) {
	// Read length (4 bytes)
	lengthBytes := s.bytePool.Get(4)
	defer s.bytePool.Put(lengthBytes)

	_, err := io.ReadFull(reader, lengthBytes)
	if err != nil {
		return nil, err
	}

	length := binary.BigEndian.Uint32(lengthBytes)
	s.stats.mutex.Lock()
	s.stats.BytesRead += uint64(length) + 4
	s.stats.mutex.Unlock()

	// Read version (1 byte)
	versionByte := s.bytePool.Get(1)
	defer s.bytePool.Put(versionByte)
	_, err = io.ReadFull(reader, versionByte)
	if err != nil {
		return nil, err
	}

	// Read command (1 byte)
	commandByte := s.bytePool.Get(1)
	defer s.bytePool.Put(commandByte)
	_, err = io.ReadFull(reader, commandByte)
	if err != nil {
		return nil, err
	}

	msg := &Message{
		Length:  length,
		Version: versionByte[0],
		Command: commandByte[0],
	}

	// Check protocol version
	if msg.Version != PROTOCOL_VERSION {
		return nil, fmt.Errorf("unsupported protocol version: %d (expected %d)", msg.Version, PROTOCOL_VERSION)
	}

	// Read remaining payload based on command
	remaining := int(length) - 2 // Subtract version and command bytes

	switch msg.Command {
	case CMD_SET:
		// Format: [keylen:4][key][ttl:4][valuelen:4][value]
		if remaining < 12 { // Minimum: keylen + ttl + valuelen
			return nil, fmt.Errorf("invalid SET message length")
		}

		// Read key length and key
		keyLenBytes := s.bytePool.Get(4)
		defer s.bytePool.Put(keyLenBytes)
		io.ReadFull(reader, keyLenBytes)
		keyLen := binary.BigEndian.Uint32(keyLenBytes)

		msg.Key = s.bytePool.Get(int(keyLen))
		io.ReadFull(reader, msg.Key)

		// Read TTL
		ttlBytes := make([]byte, 4)
		io.ReadFull(reader, ttlBytes)
		msg.TTL = binary.BigEndian.Uint32(ttlBytes)

		// Read value length and value
		valueLenBytes := make([]byte, 4)
		io.ReadFull(reader, valueLenBytes)
		valueLen := binary.BigEndian.Uint32(valueLenBytes)

		msg.Value = s.bytePool.Get(int(valueLen))
		io.ReadFull(reader, msg.Value)

	case CMD_GET, CMD_DEL, CMD_EXISTS, CMD_TTL, CMD_LLEN, CMD_SMEMBERS, CMD_SCARD, CMD_HGETALL, CMD_HLEN:
		// Format: [keylen:4][key]
		if remaining < 4 {
			return nil, fmt.Errorf("invalid message length")
		}

		keyLenBytes := make([]byte, 4)
		io.ReadFull(reader, keyLenBytes)
		keyLen := binary.BigEndian.Uint32(keyLenBytes)

		msg.Key = s.bytePool.Get(int(keyLen))
		io.ReadFull(reader, msg.Key)

	case CMD_EXPIRE:
		// Format: [keylen:4][key][ttl:4]
		if remaining < 8 {
			return nil, fmt.Errorf("invalid EXPIRE message length")
		}

		keyLenBytes := make([]byte, 4)
		io.ReadFull(reader, keyLenBytes)
		keyLen := binary.BigEndian.Uint32(keyLenBytes)

		msg.Key = make([]byte, keyLen)
		io.ReadFull(reader, msg.Key)

		ttlBytes := s.bytePool.Get(4)
		defer s.bytePool.Put(ttlBytes)
		io.ReadFull(reader, ttlBytes)
		msg.TTL = binary.BigEndian.Uint32(ttlBytes)

	case CMD_LPUSH, CMD_RPUSH, CMD_SADD:
		// Format: [keylen:4][key][valuelen:4][value]
		if remaining < 8 {
			return nil, fmt.Errorf("invalid list/set operation message length")
		}

		keyLenBytes := make([]byte, 4)
		io.ReadFull(reader, keyLenBytes)
		keyLen := binary.BigEndian.Uint32(keyLenBytes)

		msg.Key = make([]byte, keyLen)
		io.ReadFull(reader, msg.Key)

		valueLenBytes := s.bytePool.Get(4)
		defer s.bytePool.Put(valueLenBytes)
		io.ReadFull(reader, valueLenBytes)
		valueLen := binary.BigEndian.Uint32(valueLenBytes)

		msg.Value = s.bytePool.Get(int(valueLen))
		io.ReadFull(reader, msg.Value)

	case CMD_LPOP, CMD_RPOP, CMD_SREM, CMD_SISMEMBER:
		// Format: [keylen:4][key][valuelen:4][value] (for operations that need a value)
		// or just [keylen:4][key] (for LPOP/RPOP)
		keyLenBytes := make([]byte, 4)
		io.ReadFull(reader, keyLenBytes)
		keyLen := binary.BigEndian.Uint32(keyLenBytes)

		msg.Key = make([]byte, keyLen)
		io.ReadFull(reader, msg.Key)

		// Check if there's more data (for SREM, SISMEMBER)
		remainingAfterKey := remaining - 4 - int(keyLen)
		if remainingAfterKey > 0 && (msg.Command == CMD_SREM || msg.Command == CMD_SISMEMBER) {
			valueLenBytes := make([]byte, 4)
			io.ReadFull(reader, valueLenBytes)
			valueLen := binary.BigEndian.Uint32(valueLenBytes)

			msg.Value = make([]byte, valueLen)
			io.ReadFull(reader, msg.Value)
		}

	case CMD_LINDEX:
		// Format: [keylen:4][key][index:4]
		if remaining < 8 {
			return nil, fmt.Errorf("invalid LINDEX message length")
		}

		keyLenBytes := make([]byte, 4)
		io.ReadFull(reader, keyLenBytes)
		keyLen := binary.BigEndian.Uint32(keyLenBytes)

		msg.Key = make([]byte, keyLen)
		io.ReadFull(reader, msg.Key)

		indexBytes := s.bytePool.Get(4)
		defer s.bytePool.Put(indexBytes)
		io.ReadFull(reader, indexBytes)
		msg.TTL = binary.BigEndian.Uint32(indexBytes) // Reusing TTL field for index

	case CMD_LRANGE:
		// Format: [keylen:4][key][start:4][end:4]
		if remaining < 12 {
			return nil, fmt.Errorf("invalid LRANGE message length")
		}

		keyLenBytes := make([]byte, 4)
		io.ReadFull(reader, keyLenBytes)
		keyLen := binary.BigEndian.Uint32(keyLenBytes)

		msg.Key = make([]byte, keyLen)
		io.ReadFull(reader, msg.Key)

		startBytes := s.bytePool.Get(4)
		defer s.bytePool.Put(startBytes)
		io.ReadFull(reader, startBytes)
		msg.TTL = binary.BigEndian.Uint32(startBytes) // Reusing TTL for start

		endBytes := s.bytePool.Get(4)
		defer s.bytePool.Put(endBytes)
		io.ReadFull(reader, endBytes)
		// We'll store end in the first 4 bytes of Value for LRANGE
		msg.Value = s.bytePool.Get(4)
		copy(msg.Value, endBytes)

	case CMD_HSET, CMD_HGET, CMD_HDEL, CMD_HEXISTS:
		// Format: [keylen:4][key][fieldlen:4][field][valuelen:4][value] (HSET)
		// or [keylen:4][key][fieldlen:4][field] (HGET, HDEL, HEXISTS)
		if remaining < 8 {
			return nil, fmt.Errorf("invalid hash operation message length")
		}

		keyLenBytes := make([]byte, 4)
		io.ReadFull(reader, keyLenBytes)
		keyLen := binary.BigEndian.Uint32(keyLenBytes)

		msg.Key = make([]byte, keyLen)
		io.ReadFull(reader, msg.Key)

		fieldLenBytes := make([]byte, 4)
		io.ReadFull(reader, fieldLenBytes)
		fieldLen := binary.BigEndian.Uint32(fieldLenBytes)

		// Store field in TTL area temporarily (we'll parse it in processCommand)
		fieldBytes := s.bytePool.Get(int(fieldLen))
		defer s.bytePool.Put(fieldBytes)
		io.ReadFull(reader, fieldBytes)

		// For HSET, read value as well
		remainingAfterField := remaining - 8 - int(keyLen) - int(fieldLen)
		if remainingAfterField > 0 && msg.Command == CMD_HSET {
			valueLenBytes := make([]byte, 4)
			io.ReadFull(reader, valueLenBytes)
			valueLen := binary.BigEndian.Uint32(valueLenBytes)

			msg.Value = s.bytePool.Get(len(fieldBytes) + 4 + int(valueLen))
			// Pack: [fieldlen:4][field][value]
			binary.BigEndian.PutUint32(msg.Value[0:4], fieldLen)
			copy(msg.Value[4:], fieldBytes)
			io.ReadFull(reader, msg.Value[4+fieldLen:])
		} else {
			// Just field for HGET, HDEL, HEXISTS
			msg.Value = fieldBytes
		}

	case CMD_MGET:
		// Format: [count:4][key1_len:4][key1][key2_len:4][key2]...
		if remaining < 4 {
			return nil, fmt.Errorf("invalid MGET message length")
		}

		// Read the entire remaining payload as Value for parsing in handler
		msg.Value = s.bytePool.Get(remaining)
		io.ReadFull(reader, msg.Value)

	case CMD_MSET:
		// Format: [count:4][key1_len:4][key1][val1_len:4][val1][ttl1:4]...
		if remaining < 4 {
			return nil, fmt.Errorf("invalid MSET message length")
		}

		// Read the entire remaining payload as Value for parsing in handler
		msg.Value = s.bytePool.Get(remaining)
		io.ReadFull(reader, msg.Value)

	case CMD_PIPELINE:
		// Format: [count:4][msg1][msg2][msg3]...
		if remaining < 4 {
			return nil, fmt.Errorf("invalid PIPELINE message length")
		}

		// Read the entire remaining payload as Value for parsing in handler
		msg.Value = s.bytePool.Get(remaining)
		io.ReadFull(reader, msg.Value)

	case CMD_INCR, CMD_DECR:
		// Format: [keylen:4][key] (simple key-only commands)
		if remaining < 4 {
			return nil, fmt.Errorf("invalid INCR/DECR message length")
		}
		keyLenBytes := make([]byte, 4)
		io.ReadFull(reader, keyLenBytes)
		keyLen := binary.BigEndian.Uint32(keyLenBytes)

		msg.Key = make([]byte, keyLen)
		io.ReadFull(reader, msg.Key)

	case CMD_GETSET:
		// Format: [keylen:4][key][valuelen:4][value]
		if remaining < 8 {
			return nil, fmt.Errorf("invalid GETSET message length")
		}
		keyLenBytes := make([]byte, 4)
		io.ReadFull(reader, keyLenBytes)
		keyLen := binary.BigEndian.Uint32(keyLenBytes)

		msg.Key = make([]byte, keyLen)
		io.ReadFull(reader, msg.Key)

		valueLenBytes := make([]byte, 4)
		io.ReadFull(reader, valueLenBytes)
		valueLen := binary.BigEndian.Uint32(valueLenBytes)

		msg.Value = make([]byte, valueLen)
		io.ReadFull(reader, msg.Value)

	case CMD_KEYS:
		// Format: [patternlen:4][pattern]
		if remaining < 4 {
			return nil, fmt.Errorf("invalid KEYS message length")
		}
		patternLenBytes := make([]byte, 4)
		io.ReadFull(reader, patternLenBytes)
		patternLen := binary.BigEndian.Uint32(patternLenBytes)

		msg.Value = make([]byte, patternLen) // Store pattern in Value field
		io.ReadFull(reader, msg.Value)

	case CMD_SCAN:
		// Format: [cursor:4][patternlen:4][pattern]
		if remaining < 8 {
			return nil, fmt.Errorf("invalid SCAN message length")
		}
		cursorBytes := make([]byte, 4)
		io.ReadFull(reader, cursorBytes)
		msg.TTL = binary.BigEndian.Uint32(cursorBytes) // Reuse TTL field for cursor

		patternLenBytes := make([]byte, 4)
		io.ReadFull(reader, patternLenBytes)
		patternLen := binary.BigEndian.Uint32(patternLenBytes)

		msg.Value = make([]byte, patternLen)
		io.ReadFull(reader, msg.Value)

	}
	return msg, nil
}

// processCommand handles cache operations
func (s *GoFastServer) processCommand(msg *Message) []byte {
	if msg.Command != CMD_PIPELINE {
		s.incrementStat("total_ops")
	} else {
		// For pipelines, increment by the number of commands in the pipeline
		if len(msg.Value) >= 4 {
			count := binary.BigEndian.Uint32(msg.Value[0:4])
			for range count {
				s.incrementStat("total_ops")
			}
		}
	}

	key := string(msg.Key)
	now := time.Now().Unix()

	switch msg.Command {
	case CMD_SET:
		s.incrementStat("set_ops")

		item := &CacheItem{
			DataType:  TYPE_STRING,
			Value:     msg.Value,
			CreatedAt: now,
		}

		if msg.TTL > 0 {
			item.ExpiresAt = now + int64(msg.TTL)
			s.ttlMutex.Lock()
			s.ttlIndex[key] = item.ExpiresAt
			s.ttlMutex.Unlock()
		}

		s.storage.Store(key, item)
		return s.createResponse(RESP_OK, nil)

	case CMD_GET:
		s.incrementStat("get_ops")

		value, exists := s.storage.Load(key)
		if !exists {
			return s.createResponse(RESP_NOT_FOUND, nil)
		}

		item := value.(*CacheItem)

		// Check if expired
		if item.ExpiresAt > 0 && item.ExpiresAt <= now {
			s.storage.Delete(key)
			s.ttlMutex.Lock()
			delete(s.ttlIndex, key)
			s.ttlMutex.Unlock()
			return s.createResponse(RESP_NOT_FOUND, nil)
		}

		if item.DataType != TYPE_STRING {
			return s.createResponse(RESP_ERROR, []byte("WRONGTYPE Operation against a key holding the wrong kind of value"))
		}

		return s.createResponse(RESP_OK, item.Value.([]byte))

	case CMD_MGET:
		return s.handleMGet(msg.Value, now)

	case CMD_MSET:
		return s.handleMSet(msg.Value, now)

	case CMD_PIPELINE:
		return s.handlePipeline(msg.Value, now)

	// List operations
	case CMD_LPUSH:
		return s.handleListPush(key, msg.Value, true, now)

	case CMD_RPUSH:
		return s.handleListPush(key, msg.Value, false, now)

	case CMD_LPOP:
		return s.handleListPop(key, true, now)

	case CMD_RPOP:
		return s.handleListPop(key, false, now)

	case CMD_LLEN:
		return s.handleListLen(key, now)

	case CMD_LINDEX:
		return s.handleListIndex(key, int(msg.TTL), now) // TTL field reused for index

	case CMD_LRANGE:
		end := int(binary.BigEndian.Uint32(msg.Value))
		return s.handleListRange(key, int(msg.TTL), end, now)

	// Set operations
	case CMD_SADD:
		return s.handleSetAdd(key, string(msg.Value), now)

	case CMD_SREM:
		return s.handleSetRem(key, string(msg.Value), now)

	case CMD_SMEMBERS:
		return s.handleSetMembers(key, now)

	case CMD_SCARD:
		return s.handleSetCard(key, now)

	case CMD_SISMEMBER:
		return s.handleSetIsMember(key, string(msg.Value), now)

	// Hash operations
	case CMD_HSET:
		return s.handleHashSet(key, msg.Value, now)

	case CMD_HGET:
		return s.handleHashGet(key, string(msg.Value), now)

	case CMD_HDEL:
		return s.handleHashDel(key, string(msg.Value), now)

	case CMD_HGETALL:
		return s.handleHashGetAll(key, now)

	case CMD_HLEN:
		return s.handleHashLen(key, now)

	case CMD_HEXISTS:
		return s.handleHashExists(key, string(msg.Value), now)

	case CMD_DEL:
		s.incrementStat("del_ops")

		_, exists := s.storage.LoadAndDelete(key)
		if exists {
			s.ttlMutex.Lock()
			delete(s.ttlIndex, key)
			s.ttlMutex.Unlock()
			return s.createResponse(RESP_OK, []byte("1"))
		}
		return s.createResponse(RESP_OK, []byte("0"))

	case CMD_EXISTS:
		value, exists := s.storage.Load(key)
		if !exists {
			return s.createResponse(RESP_OK, []byte("0"))
		}

		item := value.(*CacheItem)
		// Check if expired
		if item.ExpiresAt > 0 && item.ExpiresAt <= now {
			s.storage.Delete(key)
			s.ttlMutex.Lock()
			delete(s.ttlIndex, key)
			s.ttlMutex.Unlock()
			return s.createResponse(RESP_OK, []byte("0"))
		}

		return s.createResponse(RESP_OK, []byte("1"))

	case CMD_EXPIRE:
		value, exists := s.storage.Load(key)
		if !exists {
			return s.createResponse(RESP_OK, []byte("0"))
		}

		item := value.(*CacheItem)
		if msg.TTL > 0 {
			item.ExpiresAt = now + int64(msg.TTL)
			s.ttlMutex.Lock()
			s.ttlIndex[key] = item.ExpiresAt
			s.ttlMutex.Unlock()
		} else {
			item.ExpiresAt = 0
			s.ttlMutex.Lock()
			delete(s.ttlIndex, key)
			s.ttlMutex.Unlock()
		}

		s.storage.Store(key, item)
		return s.createResponse(RESP_OK, []byte("1"))

	case CMD_TTL:
		value, exists := s.storage.Load(key)
		if !exists {
			return s.createResponse(RESP_OK, []byte("-2"))
		}

		item := value.(*CacheItem)
		if item.ExpiresAt == 0 {
			return s.createResponse(RESP_OK, []byte("-1")) // No expiration
		}

		ttl := item.ExpiresAt - now
		if ttl <= 0 {
			s.storage.Delete(key)
			s.ttlMutex.Lock()
			delete(s.ttlIndex, key)
			s.ttlMutex.Unlock()
			return s.createResponse(RESP_OK, []byte("-2"))
		}

		return s.createResponse(RESP_OK, []byte(fmt.Sprintf("%d", ttl)))

	case CMD_INCR:
		return s.handleIncr(key, now)

	case CMD_DECR:
		return s.handleDecr(key, now)

	case CMD_GETSET:
		return s.handleGetSet(key, msg.Value, now)

	case CMD_KEYS:
		return s.handleKeys(string(msg.Value), now)

	case CMD_SCAN:
		// Parse cursor from msg.TTL field and pattern from msg.Value
		return s.handleScan(msg.TTL, string(msg.Value), 10, now)

	default:
		return s.createResponse(RESP_ERROR, []byte("Unknown command"))
	}
}

// New processIndividualCommand() function (add after parsePipelineMessage()):
func (s *GoFastServer) processIndividualCommand(msg *Message, now int64) []byte {
	// This is the same logic as processCommand but without pipeline handling
	// and without incrementing total_ops (we'll increment it once per pipeline)

	key := string(msg.Key)

	switch msg.Command {
	case CMD_SET:
		s.incrementStat("set_ops")
		item := &CacheItem{
			DataType:  TYPE_STRING,
			Value:     msg.Value,
			CreatedAt: now,
		}
		if msg.TTL > 0 {
			item.ExpiresAt = now + int64(msg.TTL)
			s.ttlMutex.Lock()
			s.ttlIndex[key] = item.ExpiresAt
			s.ttlMutex.Unlock()
		}
		s.storage.Store(key, item)
		return s.createResponse(RESP_OK, nil)

	case CMD_GET:
		s.incrementStat("get_ops")
		value, exists := s.storage.Load(key)
		if !exists {
			return s.createResponse(RESP_NOT_FOUND, nil)
		}
		item := value.(*CacheItem)
		if item.ExpiresAt > 0 && item.ExpiresAt <= now {
			s.storage.Delete(key)
			s.ttlMutex.Lock()
			delete(s.ttlIndex, key)
			s.ttlMutex.Unlock()
			return s.createResponse(RESP_NOT_FOUND, nil)
		}
		if item.DataType != TYPE_STRING {
			return s.createResponse(RESP_ERROR, []byte("WRONGTYPE Operation against a key holding the wrong kind of value"))
		}
		return s.createResponse(RESP_OK, item.Value.([]byte))

	case CMD_DEL:
		s.incrementStat("del_ops")
		_, exists := s.storage.LoadAndDelete(key)
		if exists {
			s.ttlMutex.Lock()
			delete(s.ttlIndex, key)
			s.ttlMutex.Unlock()
			return s.createResponse(RESP_OK, []byte("1"))
		}
		return s.createResponse(RESP_OK, []byte("0"))

	case CMD_EXISTS:
		value, exists := s.storage.Load(key)
		if !exists {
			return s.createResponse(RESP_OK, []byte("0"))
		}
		item := value.(*CacheItem)
		if item.ExpiresAt > 0 && item.ExpiresAt <= now {
			s.storage.Delete(key)
			s.ttlMutex.Lock()
			delete(s.ttlIndex, key)
			s.ttlMutex.Unlock()
			return s.createResponse(RESP_OK, []byte("0"))
		}
		return s.createResponse(RESP_OK, []byte("1"))

	case CMD_EXPIRE:
		value, exists := s.storage.Load(key)
		if !exists {
			return s.createResponse(RESP_OK, []byte("0"))
		}
		item := value.(*CacheItem)
		if msg.TTL > 0 {
			item.ExpiresAt = now + int64(msg.TTL)
			s.ttlMutex.Lock()
			s.ttlIndex[key] = item.ExpiresAt
			s.ttlMutex.Unlock()
		} else {
			item.ExpiresAt = 0
			s.ttlMutex.Lock()
			delete(s.ttlIndex, key)
			s.ttlMutex.Unlock()
		}
		s.storage.Store(key, item)
		return s.createResponse(RESP_OK, []byte("1"))

	case CMD_TTL:
		value, exists := s.storage.Load(key)
		if !exists {
			return s.createResponse(RESP_OK, []byte("-2"))
		}
		item := value.(*CacheItem)
		if item.ExpiresAt == 0 {
			return s.createResponse(RESP_OK, []byte("-1"))
		}
		ttl := item.ExpiresAt - now
		if ttl <= 0 {
			s.storage.Delete(key)
			s.ttlMutex.Lock()
			delete(s.ttlIndex, key)
			s.ttlMutex.Unlock()
			return s.createResponse(RESP_OK, []byte("-2"))
		}
		return s.createResponse(RESP_OK, []byte(fmt.Sprintf("%d", ttl)))

	// List operations
	case CMD_LPUSH:
		return s.handleListPush(key, msg.Value, true, now)
	case CMD_RPUSH:
		return s.handleListPush(key, msg.Value, false, now)
	case CMD_LPOP:
		return s.handleListPop(key, true, now)
	case CMD_RPOP:
		return s.handleListPop(key, false, now)
	case CMD_LLEN:
		return s.handleListLen(key, now)

	// Set operations
	case CMD_SADD:
		return s.handleSetAdd(key, string(msg.Value), now)
	case CMD_SREM:
		return s.handleSetRem(key, string(msg.Value), now)
	case CMD_SMEMBERS:
		return s.handleSetMembers(key, now)
	case CMD_SCARD:
		return s.handleSetCard(key, now)
	case CMD_SISMEMBER:
		return s.handleSetIsMember(key, string(msg.Value), now)

	// Hash operations
	case CMD_HSET:
		return s.handleHashSet(key, msg.Value, now)
	case CMD_HGET:
		return s.handleHashGet(key, string(msg.Value), now)
	case CMD_HDEL:
		return s.handleHashDel(key, string(msg.Value), now)
	case CMD_HGETALL:
		return s.handleHashGetAll(key, now)
	case CMD_HLEN:
		return s.handleHashLen(key, now)
	case CMD_HEXISTS:
		return s.handleHashExists(key, string(msg.Value), now)

	case CMD_LINDEX:
		return s.handleListIndex(key, int(msg.TTL), now) // TTL field reused for index

	case CMD_LRANGE:
		end := int(binary.BigEndian.Uint32(msg.Value))
		return s.handleListRange(key, int(msg.TTL), end, now)

	case CMD_INCR:
		return s.handleIncr(key, now)
	case CMD_DECR:
		return s.handleDecr(key, now)
	case CMD_GETSET:
		return s.handleGetSet(key, msg.Value, now)
	case CMD_KEYS:
		return s.handleKeys(string(msg.Value), now)
	case CMD_SCAN:
		return s.handleScan(msg.TTL, string(msg.Value), 10, now)

	default:
		return s.createResponse(RESP_ERROR, []byte("Unknown command in pipeline"))
	}
}

// createResponse creates a binary response
func (s *GoFastServer) createResponse(status uint8, data []byte) []byte {
	dataLen := len(data)
	response := s.bytePool.Get(5 + dataLen)

	response[0] = status
	binary.BigEndian.PutUint32(response[1:5], uint32(dataLen))
	if dataLen > 0 {
		copy(response[5:], data)
	}

	s.stats.mutex.Lock()
	s.stats.BytesWritten += uint64(len(response))
	s.stats.mutex.Unlock()

	return response
}

// writeResponse sends response to client
func (s *GoFastServer) writeResponse(writer *bufio.Writer, response []byte) error {
	_, err := writer.Write(response)
	return err
}
