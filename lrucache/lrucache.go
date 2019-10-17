package lrucache

type queueadt interface {
	enqueueHead(data []byte) *queuenode
	enqueue(data []byte) *queuenode
	dequeue() *queuenode
	size() int
}

type queue struct {
	head, tail *queuenode
	count      int
}

type queuenode struct {
	next *queuenode
	prev *queuenode
	data []byte
	key  string
}

func (q *queue) enqueueHead(key string, data []byte) *queuenode {
	newNode := &queuenode{
		next: q.head,
		prev: nil,
		key:  key,
		data: data,
	}
	q.head = newNode
	if q.tail == nil {
		q.tail = q.head
	}
	q.count++
	return q.head
}

func (q *queue) enqueue(key string, data []byte) *queuenode {
	if q.tail != nil {
		newNode := &queuenode{
			next: nil,
			prev: q.tail,
			key:  key,
			data: data,
		}
		q.tail.next = newNode
		q.tail = newNode
	} else {
		newNode := &queuenode{
			next: nil,
			prev: nil,
			key:  key,
			data: data,
		}
		q.tail = newNode
		q.head = q.tail
	}

	q.count++

	return q.tail
}

func (q *queue) dequeue() *queuenode {
	if q.head == nil {
		return nil
	}

	oldNode := q.head
	q.head = q.head.next
	q.count--
	return oldNode
}

func (q *queue) size() int {
	return q.count
}

// add mutex if feeling unsafe
// LRUCache caching with least recently used eviction policy
type LRUCache struct {
	hash     map[string]*queuenode
	list     *queue
	capacity int
}

// Set stores the key-value pair
func (lru *LRUCache) Set(key string, val []byte) {
	if node, ok := lru.hash[key]; ok {
		// overwrite data
		node.data = val
		// update lru status
		// update references & move to head
		if node.prev != nil {
			node.prev.next = node.next
		}
		if node.next != nil {
			node.next.prev = node.prev
		}

		node.next = lru.list.head
		node.prev = nil
		lru.list.head = node
		return
	}

	lru.hash[key] = lru.list.enqueueHead(key, val)
	if lru.capacity != 0 && lru.list.size() > lru.capacity {
		deletedNode := lru.list.dequeue()
		delete(lru.hash, deletedNode.key)
	}
}

// Get returns data for the key
func (lru *LRUCache) Get(key string) []byte {
	if node, ok := lru.hash[key]; ok {
		data := node.data

		if node.prev != nil {
			node.prev.next = node.next
		}
		if node.next != nil {
			node.next.prev = node.prev
		}

		node.next = lru.list.head
		node.prev = nil
		lru.list.head = node

		return data
	}
	return nil
}

// NewLRUCache returns new empty LRU cache
func New(capacity int) *LRUCache {
	return &LRUCache{
		hash:     map[string]*queuenode{},
		list:     &queue{},
		capacity: capacity,
	}
}
