package main

import (
	"net"
	"sync"
)

// Message represents a cache operation
type Message struct {
	Length  uint32
	Version uint8
	Command uint8
	Key     []byte
	Value   []byte
	TTL     uint32 // Time to live in seconds
}

// Protocol version
const PROTOCOL_VERSION = 0x01

// Command constants
const (
	// Basic operations
	CMD_SET    = 0x01
	CMD_GET    = 0x02
	CMD_DEL    = 0x03
	CMD_EXISTS = 0x04
	CMD_EXPIRE = 0x05
	CMD_TTL    = 0x06

	CMD_MGET = 0x07
	CMD_MSET = 0x08

	CMD_PIPELINE = 0x09

	// List operations
	CMD_LPUSH  = 0x10
	CMD_RPUSH  = 0x11
	CMD_LPOP   = 0x12
	CMD_RPOP   = 0x13
	CMD_LLEN   = 0x14
	CMD_LINDEX = 0x15
	CMD_LRANGE = 0x16

	// Set operations
	CMD_SADD      = 0x20
	CMD_SREM      = 0x21
	CMD_SMEMBERS  = 0x22
	CMD_SCARD     = 0x23
	CMD_SISMEMBER = 0x24

	// Hash operations
	CMD_HSET    = 0x30
	CMD_HGET    = 0x31
	CMD_HDEL    = 0x32
	CMD_HGETALL = 0x33
	CMD_HLEN    = 0x34
	CMD_HEXISTS = 0x35

	CMD_INCR   = 0x40
	CMD_DECR   = 0x41
	CMD_GETSET = 0x42
	CMD_KEYS   = 0x43
	CMD_SCAN   = 0x44
)

// Response constants
const (
	RESP_OK        = 0x00
	RESP_ERROR     = 0x01
	RESP_NOT_FOUND = 0x02
)

// DataType represents the type of stored data
type DataType uint8

const (
	TYPE_STRING = 0x01
	TYPE_LIST   = 0x02
	TYPE_SET    = 0x03
	TYPE_HASH   = 0x04
)

// CacheItem represents a stored cache item with type information
type CacheItem struct {
	DataType  DataType
	Value     any   // Can be []byte, *List, *Set, or *Hash
	ExpiresAt int64 // Unix timestamp, 0 means no expiration
	CreatedAt int64
}

// List represents a doubly-linked list
type List struct {
	head   *ListNode
	tail   *ListNode
	length int
	mutex  sync.RWMutex
}

type ListNode struct {
	value []byte
	prev  *ListNode
	next  *ListNode
}

// Set represents a hash set
type Set struct {
	members map[string]struct{}
	mutex   sync.RWMutex
}

// Hash represents a hash map
type Hash struct {
	fields map[string][]byte
	mutex  sync.RWMutex
}

type BytePool struct {
	pool sync.Pool
}

// GoFastServer is the main server structure
type GoFastServer struct {
	storage  sync.Map         // Thread-safe storage
	ttlIndex map[string]int64 // TTL index for efficient expiration
	ttlMutex sync.RWMutex     // Protect TTL index
	stats    *ServerStats     // Performance statistics
	bytePool *BytePool        // ADD THIS LINE - Memory pool for byte slices
	listener net.Listener
	port     int
	running  bool
	config   *Config
}

// ServerStats tracks performance metrics
type ServerStats struct {
	TotalOps     uint64
	GetOps       uint64
	SetOps       uint64
	DelOps       uint64
	HitRate      float64
	BytesRead    uint64
	BytesWritten uint64
	Connections  uint64
	mutex        sync.RWMutex
}
