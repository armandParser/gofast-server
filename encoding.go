package main

import "encoding/binary"

// Encoding helpers for complex responses
func (s *GoFastServer) encodeArray(values [][]byte) []byte {
	// Simple encoding: [count:4][len1:4][val1][len2:4][val2]...
	totalLen := 4 // count field
	for _, val := range values {
		totalLen += 4 + len(val) // length + value
	}

	result := s.bytePool.Get(totalLen)
	binary.BigEndian.PutUint32(result[0:4], uint32(len(values)))

	offset := 4
	for _, val := range values {
		binary.BigEndian.PutUint32(result[offset:offset+4], uint32(len(val)))
		offset += 4
		copy(result[offset:], val)
		offset += len(val)
	}

	return result
}

func (s *GoFastServer) encodeStringArray(values []string) []byte {
	byteValues := make([][]byte, len(values))
	for i, v := range values {
		strBytes := s.bytePool.Get(len(v))
		copy(strBytes, []byte(v))
		byteValues[i] = strBytes
	}
	return s.encodeArray(byteValues)
}

func (s *GoFastServer) encodeHashMap(fields map[string][]byte) []byte {
	// Encoding: [count:4][field1_len:4][field1][val1_len:4][val1]...
	totalLen := 4 // count field
	for field, val := range fields {
		totalLen += 4 + len(field) + 4 + len(val)
	}

	result := s.bytePool.Get(totalLen)

	binary.BigEndian.PutUint32(result[0:4], uint32(len(fields)))

	offset := 4
	for field, val := range fields {
		// Field length and field
		binary.BigEndian.PutUint32(result[offset:offset+4], uint32(len(field)))
		offset += 4
		copy(result[offset:], []byte(field))
		offset += len(field)

		// Value length and value
		binary.BigEndian.PutUint32(result[offset:offset+4], uint32(len(val)))
		offset += 4
		copy(result[offset:], val)
		offset += len(val)
	}

	return result
}

func (s *GoFastServer) encodeMGetResponse(values [][]byte) []byte {
	// Encoding: [count:4][val1_len:4][val1][val2_len:4][val2]... (nil values have len=0xFFFFFFFF)
	totalLen := 4 // count field
	for _, val := range values {
		if val == nil {
			totalLen += 4 // Just the length field for nil
		} else {
			totalLen += 4 + len(val) // length + value
		}
	}

	result := s.bytePool.Get(totalLen)
	binary.BigEndian.PutUint32(result[0:4], uint32(len(values)))

	offset := 4
	for _, val := range values {
		if val == nil {
			// Use 0xFFFFFFFF to indicate nil/not found
			binary.BigEndian.PutUint32(result[offset:offset+4], 0xFFFFFFFF)
			offset += 4
		} else {
			binary.BigEndian.PutUint32(result[offset:offset+4], uint32(len(val)))
			offset += 4
			copy(result[offset:], val)
			offset += len(val)
		}
	}

	return result
}

// New encodePipelineResponse() function (add after encoding helpers):
func (s *GoFastServer) encodePipelineResponse(responses [][]byte) []byte {
	// Encoding: [count:4][resp1][resp2][resp3]...
	totalLen := 4 // count field
	for _, resp := range responses {
		totalLen += len(resp)
	}

	result := s.bytePool.Get(totalLen)
	binary.BigEndian.PutUint32(result[0:4], uint32(len(responses)))

	offset := 4
	for _, resp := range responses {
		copy(result[offset:], resp)
		offset += len(resp)
	}

	return result
}

func (s *GoFastServer) encodeScanResponse(cursor uint32, keys []string) []byte {
	// SCAN response format: [cursor:4][count:4][key1_len:4][key1][key2_len:4][key2]...
	totalLen := 4 + 4 // cursor + count
	for _, key := range keys {
		totalLen += 4 + len(key) // keylen + key
	}

	result := s.bytePool.Get(totalLen)

	// Write cursor
	binary.BigEndian.PutUint32(result[0:4], cursor)

	// Write count
	binary.BigEndian.PutUint32(result[4:8], uint32(len(keys)))

	// Write keys
	offset := 8
	for _, key := range keys {
		keyBytes := []byte(key)
		binary.BigEndian.PutUint32(result[offset:offset+4], uint32(len(keyBytes)))
		offset += 4
		copy(result[offset:], keyBytes)
		offset += len(keyBytes)
	}

	return result
}
