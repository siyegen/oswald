package main

import (
	"fmt"
	"time"

	"github.com/boltdb/bolt"
)

const UID_BUCKET string = "__oswald_uid"

type PomStore interface {
	StoreStatus(status string, pom *Pom) error
	// GetStatus(status string) error
	GetStatusCount(status string) (int, error) // TODO: Replace status with type
	Clear() error
}

type BoltPomStore struct {
	uid    []byte
	db     *bolt.DB
	dbName string
}

func createUser(db *bolt.DB) ([]byte, error) {
	var uid []byte
	uidKey := []byte(UID_BUCKET)
	fmt.Println("Using uidKey", string(uidKey))
	// TODO: See if we can clean this up or move out
	err := db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte(UID_BUCKET))
		if err != nil {
			return err
		}
		if existingUid := bucket.Get(uidKey); existingUid != nil {
			fmt.Println("has existingUid", string(existingUid))
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
		fmt.Errorf("Error opening db %s", err)
	}
	uid, err := createUser(db)
	if err != nil {
		fmt.Errorf("Error creating/storing uid %s", err)
	}
	fmt.Println("uuid:", string(uid))
	return &BoltPomStore{db: db, dbName: name, uid: uid}
}

func (b *BoltPomStore) Clear() error {
	fmt.Println("uid before", string(b.uid))
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
	fmt.Println("All deletes worked")
	if err != nil {
		return err
	}
	newUid, err := createUser(b.db)
	fmt.Println("newuid", string(newUid))
	if err != nil {
		return err
	}
	b.uid = newUid
	fmt.Println("uid after", string(b.uid))
	return nil
}

// TODO: Implement these
func (b *BoltPomStore) StoreStatus(status string, pom *Pom) error { // REVIEW: Should pom be pomEvent?
	return b.db.Update(func(tx *bolt.Tx) error {
		fmt.Println("using uuid", string(b.GetUid()), "for", status)
		bucket, err := tx.CreateBucketIfNotExists(b.GetUid()) // TODO: Really, clean this up...
		if err != nil {
			fmt.Println("Couldn't create bucket", status)
			return err
		}
		statusBucket, err := bucket.CreateBucketIfNotExists([]byte(status))
		if err != nil {
			fmt.Println("Error with bucket storeStatus", status)
			return err
		}
		nextId, _ := statusBucket.NextSequence()
		sortableTime := []byte(pom.startTime.Format(time.RFC3339))
		fmt.Println("Storing...", nextId)
		return statusBucket.Put(sortableTime, itob(int(nextId)))
	})
}

func (b *BoltPomStore) GetStatusCount(status string) (int, error) {
	count := 0
	b.db.View(func(tx *bolt.Tx) error {
		fmt.Println("using uuid", string(b.GetUid()), "for", status)
		bucket := tx.Bucket(b.GetUid())
		fmt.Println("getstatuscount for", status)
		if bucket == nil {
			fmt.Println("no uid bucket for", status)
			return nil // TODO: change to rich return
		}
		statusBucket := bucket.Bucket([]byte(status))
		if statusBucket == nil { // Assume no count
			fmt.Println("no bucket for", status)
			return nil
		}
		key, value := statusBucket.Cursor().Last()
		fmt.Println("key, value", string(key), value)
		count = btoi(value)
		return nil
	})
	return count, nil
}
