package main

import (
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

// DamageConfig holds all impairment parameters
type DamageConfig struct {
	LatencyMs        int     `json:"latencyMs"`
	JitterMs         int     `json:"jitterMs"`
	PacketLossPct    float64 `json:"packetLossPct"`
	DuplicatePct     float64 `json:"duplicatePct"`
	ReorderPct       float64 `json:"reorderPct"`
	TamperPct        float64 `json:"tamperPct"`
	ThrottleKbps     int     `json:"throttleKbps"`
	Direction        string  `json:"direction"`   // "both", "outbound", "inbound"
	Protocol         string  `json:"protocol"`    // "any", "tcp", "udp", "icmp"
	PortFilter       string  `json:"portFilter"`
	IpFilter         string  `json:"ipFilter"`
}

// Stats holds thread-safe processing statistics
type Stats struct {
	Processed atomic.Int64
	Bytes     atomic.Int64
	Delayed   atomic.Int64
	Dropped   atomic.Int64
	Duplicated atomic.Int64
	Reordered atomic.Int64
	Tampered  atomic.Int64
}

// DelayedPacket is a packet waiting to be sent
type DelayedPacket struct {
	Data      []byte
	Addr      WinDivertAddress
	Deadline  int64 // unix nano
}

// PacketQueue is a thread-safe priority queue for delayed packets
type PacketQueue struct {
	mu    sync.Mutex
	queue []*DelayedPacket
}

func NewPacketQueue() *PacketQueue {
	return &PacketQueue{}
}

func (q *PacketQueue) Enqueue(p *DelayedPacket) {
	q.mu.Lock()
	defer q.mu.Unlock()
	// Insert sorted by deadline
	n := len(q.queue)
	idx := n
	for i := 0; i < n; i++ {
		if q.queue[i].Deadline > p.Deadline {
			idx = i
			break
		}
	}
	q.queue = append(q.queue, nil)
	copy(q.queue[idx+1:], q.queue[idx:])
	q.queue[idx] = p
}

func (q *PacketQueue) DequeueReady() []*DelayedPacket {
	q.mu.Lock()
	defer q.mu.Unlock()
	now := time.Now().UnixNano()
	var ready []*DelayedPacket
	for len(q.queue) > 0 && q.queue[0].Deadline <= now {
		ready = append(ready, q.queue[0])
		q.queue = q.queue[1:]
	}
	return ready
}

func (q *PacketQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.queue)
}

func (q *PacketQueue) Clear() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.queue = q.queue[:0]
}

// TokenBucket for bandwidth throttling
type TokenBucket struct {
	mu       sync.Mutex
	tokens   float64
	maxRate  float64 // bytes per ms
	lastTime int64
}

func NewTokenBucket() *TokenBucket {
	return &TokenBucket{lastTime: time.Now().UnixNano()}
}

func (tb *TokenBucket) Configure(kbps int) {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	tb.maxRate = float64(kbps) * 1024.0 / 8.0 / 1000.0 // bytes per ms
	tb.tokens = 0
	tb.lastTime = time.Now().UnixNano()
}

func (tb *TokenBucket) TryConsume(size int) bool {
	if tb.maxRate <= 0 {
		return true
	}
	tb.mu.Lock()
	defer tb.mu.Unlock()
	now := time.Now().UnixNano()
	elapsed := float64(now-tb.lastTime) / float64(time.Millisecond)
	tb.lastTime = now
	tb.tokens = min(tb.maxRate*100, tb.tokens+elapsed*tb.maxRate) // cap at 100ms worth
	if tb.tokens >= float64(size) {
		tb.tokens -= float64(size)
		return true
	}
	return false
}

func (tb *TokenBucket) Reset() {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	tb.tokens = 0
	tb.lastTime = time.Now().UnixNano()
}

// DamageEngine is the core packet processing engine
type DamageEngine struct {
	handle       uintptr
	config       DamageConfig
	stats        Stats
	delayQueue   *PacketQueue
	reorderQueue *PacketQueue
	throttleQueue *PacketQueue
	tokenBucket  *TokenBucket
	stopCh       chan struct{}
	running      bool
}

func NewDamageEngine() *DamageEngine {
	return &DamageEngine{
		delayQueue:    NewPacketQueue(),
		reorderQueue:  NewPacketQueue(),
		throttleQueue: NewPacketQueue(),
		tokenBucket:   NewTokenBucket(),
		stopCh:        make(chan struct{}),
	}
}

func (e *DamageEngine) Start(handle uintptr, config DamageConfig) error {
	e.handle = handle
	e.config = config
	e.tokenBucket.Configure(config.ThrottleKbps)
	e.running = true
	e.stopCh = make(chan struct{})

	go e.delayQueueLoop()
	go e.throttleQueueLoop()
	return nil
}

func (e *DamageEngine) Stop() {
	if !e.running {
		return
	}
	e.running = false
	close(e.stopCh)
	e.delayQueue.Clear()
	e.reorderQueue.Clear()
	e.throttleQueue.Clear()
	e.tokenBucket.Reset()
}

func (e *DamageEngine) UpdateConfig(config DamageConfig) {
	e.config = config
	e.tokenBucket.Configure(config.ThrottleKbps)
}

// ProcessPacket applies all damage rules to a received packet
func (e *DamageEngine) ProcessPacket(data []byte, addr WinDivertAddress) {
	e.stats.Processed.Add(1)
	e.stats.Bytes.Add(int64(len(data)))

	// 1. Packet loss
	if e.config.PacketLossPct > 0 && rand.Float64()*100 < e.config.PacketLossPct {
		e.stats.Dropped.Add(1)
		return
	}

	// 2. Tamper
	if e.config.TamperPct > 0 && rand.Float64()*100 < e.config.TamperPct {
		e.tamperPacket(data)
		e.stats.Tampered.Add(1)
	}

	// 3. Duplicate
	if e.config.DuplicatePct > 0 && rand.Float64()*100 < e.config.DuplicatePct {
		dup := make([]byte, len(data))
		copy(dup, data)
		e.delayQueue.Enqueue(&DelayedPacket{
			Data:     dup,
			Addr:     addr,
			Deadline: time.Now().Add(5 * time.Millisecond).UnixNano(),
		})
		e.stats.Duplicated.Add(1)
	}

	// 4. Reorder
	if e.config.ReorderPct > 0 && rand.Float64()*100 < e.config.ReorderPct {
		delay := time.Duration(10+rand.Intn(50)) * time.Millisecond
		e.reorderQueue.Enqueue(&DelayedPacket{
			Data:     data,
			Addr:     addr,
			Deadline: time.Now().Add(delay).UnixNano(),
		})
		e.stats.Reordered.Add(1)
		return
	}

	// 5. Delay + Jitter
	if e.config.LatencyMs > 0 || e.config.JitterMs > 0 {
		jitter := 0
		if e.config.JitterMs > 0 {
			jitter = rand.Intn(e.config.JitterMs*2+1) - e.config.JitterMs
		}
		delay := e.config.LatencyMs + jitter
		if delay > 0 {
			e.delayQueue.Enqueue(&DelayedPacket{
				Data:     data,
				Addr:     addr,
				Deadline: time.Now().Add(time.Duration(delay) * time.Millisecond).UnixNano(),
			})
			e.stats.Delayed.Add(1)
			return
		}
	}

	// 6. Throttle
	if e.config.ThrottleKbps > 0 && !e.tokenBucket.TryConsume(len(data)) {
		e.throttleQueue.Enqueue(&DelayedPacket{
			Data:     data,
			Addr:     addr,
			Deadline: time.Now().Add(time.Millisecond).UnixNano(),
		})
		return
	}

	// Send immediately
	winDivertSend(e.handle, data, &addr)
}

func (e *DamageEngine) delayQueueLoop() {
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-e.stopCh:
			return
		case <-ticker.C:
			for _, pkt := range e.delayQueue.DequeueReady() {
				winDivertSend(e.handle, pkt.Data, &pkt.Addr)
			}
			for _, pkt := range e.reorderQueue.DequeueReady() {
				// After reorder, send through delay
				if e.config.LatencyMs > 0 || e.config.JitterMs > 0 {
					jitter := 0
					if e.config.JitterMs > 0 {
						jitter = rand.Intn(e.config.JitterMs*2+1) - e.config.JitterMs
					}
					delay := e.config.LatencyMs + jitter
					if delay > 0 {
						e.delayQueue.Enqueue(&DelayedPacket{
							Data:     pkt.Data,
							Addr:     pkt.Addr,
							Deadline: time.Now().Add(time.Duration(delay) * time.Millisecond).UnixNano(),
						})
						e.stats.Delayed.Add(1)
						continue
					}
				}
				winDivertSend(e.handle, pkt.Data, &pkt.Addr)
			}
		}
	}
}

func (e *DamageEngine) throttleQueueLoop() {
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-e.stopCh:
			return
		case <-ticker.C:
			for _, pkt := range e.throttleQueue.DequeueReady() {
				if e.config.ThrottleKbps > 0 && !e.tokenBucket.TryConsume(len(pkt.Data)) {
					e.throttleQueue.Enqueue(&DelayedPacket{
						Data:     pkt.Data,
						Addr:     pkt.Addr,
						Deadline: time.Now().Add(time.Millisecond).UnixNano(),
					})
					continue
				}
				winDivertSend(e.handle, pkt.Data, &pkt.Addr)
			}
		}
	}
}

func (e *DamageEngine) tamperPacket(data []byte) {
	if len(data) <= 24 {
		return
	}
	flips := 1 + rand.Intn(3)
	for i := 0; i < flips; i++ {
		offset := 20 + rand.Intn(len(data)-20)
		data[offset] ^= byte(rand.Intn(256))
	}
}
