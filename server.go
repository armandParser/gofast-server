package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"time"
)

func (s *GoFastServer) SetConfig(config *Config) {
	s.config = config
}

func NewGoFastServer(port int) *GoFastServer {
	return &GoFastServer{
		port:     port,
		ttlIndex: make(map[string]int64),
		stats:    &ServerStats{},
		bytePool: NewBytePool(),
		config:   nil, // Will be set later
	}
}

// Start begins listening for connections
func (s *GoFastServer) Start() error {
	var err error

	// Use config host if available, otherwise default to localhost
	host := "localhost"
	if s.config != nil {
		host = s.config.Host
	}

	address := fmt.Sprintf("%s:%d", host, s.port)
	s.listener, err = net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("failed to start server: %v", err)
	}

	s.running = true
	log.Printf("GoFast server started on %s", address)

	// Start background cleanup goroutine
	go s.cleanupExpiredKeys()

	// Accept connections
	for s.running {
		conn, err := s.listener.Accept()
		if err != nil {
			if s.running {
				log.Printf("Accept error: %v", err)
			}
			continue
		}

		// Handle connection in goroutine
		go s.handleConnection(conn)
		s.incrementStat("connections")
	}

	return nil
}

// Stop gracefully shuts down the server
func (s *GoFastServer) Stop() {
	s.running = false
	if s.listener != nil {
		s.listener.Close()
	}
}

// handleConnection processes client connections
func (s *GoFastServer) handleConnection(conn net.Conn) {
	defer conn.Close()

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	for {
		// Read message from client
		msg, err := s.readMessage(reader)
		if err != nil {
			if err != io.EOF {
				log.Printf("Read error: %v", err)
			}
			break
		}

		// Process the command
		response := s.processCommand(msg)

		// Send response
		err = s.writeResponse(writer, response)
		if err != nil {
			log.Printf("Write error: %v", err)
			break
		}

		writer.Flush()
	}
}

func (s *GoFastServer) cleanupExpiredKeys() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for s.running {
		<-ticker.C
		now := time.Now().Unix()
		s.ttlMutex.Lock()

		var expiredKeys []string
		for key, expiresAt := range s.ttlIndex {
			if expiresAt <= now {
				expiredKeys = append(expiredKeys, key)
			}
		}

		for _, key := range expiredKeys {
			s.storage.Delete(key)
			delete(s.ttlIndex, key)
		}

		s.ttlMutex.Unlock()

		if len(expiredKeys) > 0 {
			log.Printf("Cleaned up %d expired keys", len(expiredKeys))
		}
	}
}
