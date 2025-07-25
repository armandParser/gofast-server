package main

// incrementStat atomically increments a statistic
func (s *GoFastServer) incrementStat(stat string) {
	s.stats.mutex.Lock()
	defer s.stats.mutex.Unlock()

	switch stat {
	case "total_ops":
		s.stats.TotalOps++
	case "get_ops":
		s.stats.GetOps++
	case "set_ops":
		s.stats.SetOps++
	case "del_ops":
		s.stats.DelOps++
	case "connections":
		s.stats.Connections++
	}
}

// GetStats returns current server statistics
func (s *GoFastServer) GetStats() *ServerStats {
	s.stats.mutex.RLock()
	defer s.stats.mutex.RUnlock()

	// Calculate hit rate
	if s.stats.GetOps > 0 {
		// This is a simplified hit rate calculation
		// In practice, you'd track hits vs misses separately
		s.stats.HitRate = float64(s.stats.GetOps-s.stats.DelOps) / float64(s.stats.GetOps)
	}

	// Return a copy to avoid race conditions
	return &ServerStats{
		TotalOps:     s.stats.TotalOps,
		GetOps:       s.stats.GetOps,
		SetOps:       s.stats.SetOps,
		DelOps:       s.stats.DelOps,
		HitRate:      s.stats.HitRate,
		BytesRead:    s.stats.BytesRead,
		BytesWritten: s.stats.BytesWritten,
		Connections:  s.stats.Connections,
	}
}
