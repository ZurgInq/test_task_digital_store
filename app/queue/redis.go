package queue

import (
	"sync"
	"time"
)

// simple mock by chatGPT
type RedisMock struct {
	mu     sync.Mutex
	queues map[string][]string
	locks  map[string]lock
}

type lock struct {
	value     string
	expiresAt time.Time
}

func NewRedisMock() *RedisMock {
	return &RedisMock{
		queues: make(map[string][]string),
		locks:  make(map[string]lock),
	}
}

// LPush добавляет запись в начало очереди.
func (r *RedisMock) LPush(queue string, value string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.queues[queue] = append([]string{value}, r.queues[queue]...)
}

// RPush добавляет запись в конец очереди.
func (r *RedisMock) RPush(queue string, value string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.queues[queue] = append(r.queues[queue], value)
}

// LPop получает и удаляет запись с начала очереди.
func (r *RedisMock) LPop(queue string) (string, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	items := r.queues[queue]
	if len(items) == 0 {
		return "", false
	}

	value := items[0]
	r.queues[queue] = items[1:]

	return value, true
}

// RPop получает и удаляет запись с конца очереди.
func (r *RedisMock) RPop(queue string) (string, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	items := r.queues[queue]
	if len(items) == 0 {
		return "", false
	}

	value := items[len(items)-1]
	r.queues[queue] = items[:len(items)-1]

	return value, true
}

// SetLock устанавливает лок.
// Если лок уже существует и не истёк, возвращает false.
func (r *RedisMock) SetLock(key string, value string, ttl time.Duration) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if existing, ok := r.locks[key]; ok {
		if existing.expiresAt.IsZero() || time.Now().Before(existing.expiresAt) {
			return false
		}
	}

	r.locks[key] = lock{
		value:     value,
		expiresAt: time.Now().Add(ttl),
	}

	return true
}

// GetLock получает значение лока.
// Если лок отсутствует или истёк, возвращает false.
func (r *RedisMock) GetLock(key string) (string, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, ok := r.locks[key]
	if !ok {
		return "", false
	}

	if !existing.expiresAt.IsZero() && time.Now().After(existing.expiresAt) {
		delete(r.locks, key)
		return "", false
	}

	return existing.value, true
}

// DeleteLock снимает лок.
func (r *RedisMock) DeleteLock(key string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.locks[key]; !ok {
		return false
	}

	delete(r.locks, key)
	return true
}
