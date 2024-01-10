package database

import (
	"github.com/chen3feng/stl4go"
	"gmail-deleter/internal/models"
	"go.mongodb.org/mongo-driver/bson"

	"sync"
)

type Key []byte

type ItemList struct {
	l    *stl4go.DList[Key]
	lock sync.Mutex
}

func ItemListFactory() *ItemList {
	return &ItemList{l: stl4go.NewDList[Key]()}
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

func (i *ItemList) Pop() Key {
	i.lock.Lock()
	defer i.lock.Unlock()
	if i.l.Len() == 0 {
		return nil
	}
	item, valid := i.l.PopFront()
	if !valid {
		return nil
	}
	return item
}

func (i *ItemList) Push(threadId Key) {
	i.lock.Lock()
	defer i.lock.Unlock()
	i.l.PushFront(threadId)
}

func (i *ItemList) Count() int {
	i.lock.Lock()
	defer i.lock.Unlock()
	return i.l.Len()
}
