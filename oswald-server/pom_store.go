package main

import (
	"fmt"
	"time"

	"github.com/boltdb/bolt"
)

const UID_BUCKET string = "__oswald_uid"

type PomStore interface {
	StoreStatus(status StatusString, pom PomEvent) error
	GetStatusCount(status StatusString) (int, error)
	Clear() error
}

type BoltPomStore struct {
	userId string
	db     *bolt.DB
	dbName string
}

func createUser(db *bolt.DB) ([]byte, error) {
	var uid []byte
	uidKey := []byte(UID_BUCKET)
	err := db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists(uidKey)
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

func NewBoltPomStore(dbLocation, dbName string) PomStore {
	name := fmt.Sprintf("%s/%s", dbLocation, dbName)
	db, err := bolt.Open(name, 0600, nil)
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
		userId := []byte(b.userId)
		tx.DeleteBucket([]byte(SUCCESS))
		tx.DeleteBucket([]byte(CANCELLED))
		tx.DeleteBucket([]byte(PAUSED))
		tx.DeleteBucket(uidKey)
		tx.DeleteBucket(userId)
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

func (b *BoltPomStore) StoreStatus(status StatusString, pom PomEvent) error {
	err := b.db.Update(func(tx *bolt.Tx) error {
		userId := []byte(b.userId)
		bucket, err := tx.CreateBucketIfNotExists(userId)
		if err != nil {
			return err
		}
		statusBucket, err := bucket.CreateBucketIfNotExists([]byte(status))
		if err != nil {
			return err
		}
		nextId, _ := statusBucket.NextSequence()
		sortableTime := []byte(pom.time.Format(time.RFC3339))
		return statusBucket.Put(sortableTime, itob(int(nextId)))
	})
	return err
}

// NOTE: Read-only transaction, will never be blocked but can be off
// based on whether or not a write transaction is ongoing
func (b *BoltPomStore) GetStatusCount(status StatusString) (int, error) {
	count := 0
	err := b.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(b.userId))
		if bucket == nil {
			return nil
		}
		statusBucket := bucket.Bucket([]byte(status))
		if statusBucket == nil {
			return nil
		}
		_, value := statusBucket.Cursor().Last()
		count = btoi(value)
		return nil
	})
	return count, err
}
