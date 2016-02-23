package main

import (
	"fmt"
	"sync"
	"time"

	"github.com/boltdb/bolt"
)

const UID_BUCKET string = "__oswald_uid"

type PomStore interface {
	StoreStatus(status StatusString, pom Pom) error
	// GetStatus(status string) error
	GetStatusCount(status StatusString) (int, error)
	Clear() error
}

type BoltPomStore struct {
	userId string
	db     *bolt.DB
	dbName string
	sync.Mutex
}

func createUser(db *bolt.DB) ([]byte, error) {
	var uid []byte
	uidKey := []byte(UID_BUCKET)
	err := db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte(UID_BUCKET))
		if err != nil {
			return err
		}
		if existingUid := bucket.Get(uidKey); existingUid != nil {
			uid = existingUid
		} else {
			uid = []byte(newUUID())
			bucket.Put(uidKey, uid)
		}
		_, err = tx.CreateBucketIfNotExists(uid)
		return err
	})
	return uid, err
}

func NewBoltPomStore() PomStore {
	name := "_dev.db"
	db, err := bolt.Open(fmt.Sprintf("dev_db/%s", name), 0600, nil)
	if err != nil {
		logger.Fatalf("Error opening db %s", err)
	}
	uid, err := createUser(db)
	if err != nil {
		logger.Fatalf("Error creating uid", err)
	}
	return &BoltPomStore{db: db, dbName: name, userId: string(uid)}
}

func (b *BoltPomStore) Clear() error {
	// TODO: Some error checking
	err := b.db.Update(func(tx *bolt.Tx) error {
		uidKey := []byte(UID_BUCKET)
		logger.Println("Using uidKey", string(uidKey))
		tx.DeleteBucket([]byte(SUCCESS))
		logger.Println("deleted success")
		tx.DeleteBucket([]byte(CANCELLED))
		logger.Println("deleted cancelled")
		tx.DeleteBucket([]byte(PAUSED))
		logger.Println("deleted paused")
		tx.DeleteBucket(uidKey)
		logger.Println("deleted uidkey")
		tx.DeleteBucket([]byte(b.userId))
		logger.Println("Deleted uid")
		return nil
	})
	if err != nil {
		return err
	}
	newUid, err := createUser(b.db)
	if err != nil {
		return err
	}
	b.userId = string(newUid)
	return nil
}

func (b *BoltPomStore) StoreStatus(status StatusString, pom Pom) error { // REVIEW: Should pom be pomEvent?
	// b.Lock()
	err := b.db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte(b.userId))
		if err != nil {
			return err
		}
		statusBucket, err := bucket.CreateBucketIfNotExists([]byte(status))
		if err != nil {
			return err
		}
		nextId, _ := statusBucket.NextSequence()
		sortableTime := []byte(pom.startTime.Format(time.RFC3339))
		return statusBucket.Put(sortableTime, itob(int(nextId)))
	})
	// b.Unlock()
	return err
}

// NOTE: Read-only transaction, will never be blocked but can be off
// based on whether or not a write transaction is ongoing
func (b *BoltPomStore) GetStatusCount(status StatusString) (int, error) {
	count := 0
	// b.Lock()
	err := b.db.Update(func(tx *bolt.Tx) error {

		bucket := tx.Bucket([]byte(b.userId))
		if bucket == nil {
			logger.Println("Got a nil uid bucket for", b.userId)
			return nil // TODO: change to rich return
		}
		statusBucket := bucket.Bucket([]byte(status))
		if statusBucket == nil { // Assume no count
			logger.Println("Got a nil status bucket for", status)
			return nil
		}
		_, value := statusBucket.Cursor().Last()
		count = btoi(value)
		return nil
	})
	// b.Unlock()
	return count, err
}
