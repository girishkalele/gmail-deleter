package database

import (
	"container/list"
	"gmail-deleter/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"sync"
)

type ItemList struct {
	l    list.List
	lock sync.Mutex
}

type Database interface {
	Init()
	Close()
	ReserveWindow(int) bool
	Summarize() []models.Report
	Create(models.Thread) error
	Populate(models.Thread) error
	DeleteOne(string)
	FindOne(bson.M, string) models.Thread
	Count(bson.M) *ItemList
	MoveToDeleting([]byte) models.Thread
}

func (i *ItemList) Pop() []byte {
	i.lock.Lock()
	defer i.lock.Unlock()
	if i.l.Len() == 0 {
		return nil
	}
	item := i.l.Front()
	i.l.Remove(item)
	return item.Value.([]byte)
}

func (i *ItemList) Push(threadId []byte) {
	i.lock.Lock()
	defer i.lock.Unlock()
	i.l.PushFront(threadId)
}

func (i *ItemList) Count() int {
	i.lock.Lock()
	defer i.lock.Unlock()
	return i.l.Len()
}
