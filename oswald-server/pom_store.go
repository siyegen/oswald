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
	GetStatusCount(status StatusString) (int, error) // TODO: Replace status with type
	Clear() error
}

type BoltPomStore struct {
	uid    []byte
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
		return nil
	})
	return uid, err
}

func (b *BoltPomStore) GetUid() []byte {
	return b.uid
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
	return &BoltPomStore{db: db, dbName: name, uid: uid}
}

func (b *BoltPomStore) Clear() error {
	// TODO: Some error checking
	b.Lock()
	err := b.db.Update(func(tx *bolt.Tx) error {
		uidKey := []byte(UID_BUCKET)
		fmt.Println("Using uidKey", string(uidKey))
		tx.DeleteBucket([]byte(SUCCESS))
		fmt.Println("deleted success")
		tx.DeleteBucket([]byte(CANCELLED))
		fmt.Println("deleted cancelled")
		tx.DeleteBucket([]byte(PAUSED))
		fmt.Println("deleted paused")
		tx.DeleteBucket(uidKey)
		fmt.Println("deleted uidkey")
		tx.DeleteBucket(b.GetUid())
		fmt.Println("Deleted uid")
		return nil
	})
	b.Unlock()
	if err != nil {
		return err
	}
	newUid, err := createUser(b.db)
	if err != nil {
		return err
	}
	b.uid = newUid
	return nil
}

func (b *BoltPomStore) StoreStatus(status StatusString, pom Pom) error { // REVIEW: Should pom be pomEvent?
	b.Lock()
	err := b.db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists(b.GetUid())
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
	fmt.Println("after storestatus")
	b.Unlock()
	return err
}

// NOTE: Read-only transaction, will never be blocked but can be off
// based on whether or not a write transaction is ongoing
func (b *BoltPomStore) GetStatusCount(status StatusString) (int, error) {
	count := 0
	b.Lock()
	b.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(b.GetUid())
		if bucket == nil {
			fmt.Println("Got a nil uid bucket for", string(b.GetUid()))
			return nil // TODO: change to rich return
		}
		statusBucket := bucket.Bucket([]byte(status))
		if statusBucket == nil { // Assume no count
			fmt.Println("Got a nil status bucket for", status)
			return nil
		}
		_, value := statusBucket.Cursor().Last()
		count = btoi(value)
		fmt.Println("uid", string(b.GetUid()))
		fmt.Printf("value %+v, count %d for statusBucket %s\n\n", value, count, status)
		return nil
	})
	b.Unlock()
	return count, nil
}
