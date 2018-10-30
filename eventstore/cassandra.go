package eventstore

import (
	"context"
	"fmt"
	"time"

	"github.com/gocql/gocql"
	"github.com/pkg/errors"
	// "github.com/tysontate/rendezvous"
)

const cassCreateTableStmt = `CREATE TABLE IF NOT EXISTS event_store (aggregateStream TEXT, aggregateId TEXT, eventId TEXT, eventType TEXT, version BIGINT, data BLOB, metadata BLOB, createTime TIMESTAMP, streamCtr BIGINT STATIC, PRIMARY KEY((aggregateStream, aggregateId), version)) WITH CLUSTERING ORDER BY (version ASC)`
const cassStreamCtrQueryStmt = `SELECT streamCtr FROM event_store WHERE aggregateStream=? AND aggregateId=? LIMIT 1`
const cassInsertEventStmt = `INSERT into event_store (aggregateStream, aggregateId, eventId, eventType, version, data, metadata, createTime, streamCtr) VALUES ( ?, ?, ?, ?, ?, ?, ?, ?, ?)`
const cassEventLookupStmt = `SELECT aggregateStream, aggregateId, eventId, eventType, version, data, metadata, createTime, streamCtr from event_store WHERE aggregateStream=? AND aggregateId=? AND version >= ? LIMIT %d`

type cassandraEventStore struct {
	session *gocql.Session
}

// NewCassandraEventStore created a EventStore backed by cassandra
func NewCassandraEventStore(hosts []string, keyspace string) (EventStore, error) {
	cluster := gocql.NewCluster(hosts...)
	cluster.Keyspace = keyspace
	session, err := cluster.CreateSession()
	if err != nil {
		return nil, errors.WithMessage(err, "unable to connect to cassandra cluster")
	}

	if err := session.Query(cassCreateTableStmt).Exec(); err != nil {
		return nil, errors.WithMessage(err, "unable to create table for event storage")
	}

	return &cassandraEventStore{session: session}, nil
}

func (es *cassandraEventStore) AppendToStream(aggregateStream string, aggregateID string, expectedVersion EventVersion, events []EventData) error {
	q := es.session.Query(cassStreamCtrQueryStmt, aggregateStream, aggregateID)
	defer q.Release()

	var curVersion int64
	iter := q.Iter()
	defer iter.Close()
	if iter.NumRows() == 0 {
		curVersion = -2
	} else {
		if ok := iter.Scan(&curVersion); !ok {
			return errors.New("unable to retrieve current version of stream")
		}
	}
	iter.Close()
	q.Release()

	ts := time.Now()
	b := es.session.NewBatch(gocql.LoggedBatch).WithContext(context.Background())
	b.SetConsistency(gocql.LocalQuorum)

	switch expectedVersion {
	case ExpectedVersionNoStream:
		if curVersion != -2 {
			return errors.New("stream exists")
		}
		b.Query(cassInsertEventStmt,
			aggregateStream, aggregateID,
			"-1", "StreamStart", -1,
			"", "",
			ts, -1,
		)
		expectedVersion = 0
	case ExpectedVersionEmptyStream:
		if curVersion != -1 {
			return errors.New("stream not empty")
		}
		expectedVersion = 0
	case ExpectedVersionStreamExists:
		if curVersion == -2 {
			return errors.New("stream does not exist")
		}
		expectedVersion = EventVersion(curVersion + 1)
	case ExpectedVersionAny:
		expectedVersion = EventVersion(curVersion + 1)
	default:
	}

	for i, e := range events {
		b.Query(cassInsertEventStmt,
			aggregateStream, aggregateID,
			e.ID, e.Type, uint64(expectedVersion)+uint64(i),
			e.Data, e.Metadata,
			ts, uint64(expectedVersion)+uint64(i),
		)
	}

	err := es.session.ExecuteBatch(b)
	if err != nil {
		return errors.WithMessage(err, "error while saving changes")
	}

	return nil
}

func (es *cassandraEventStore) ReadEventStream(aggregateStream string, aggregateID string, startEventNumber EventVersion, len uint32) (RecordedEvents, error) {
	stmt := fmt.Sprintf(cassEventLookupStmt, len)
	q := es.session.Query(stmt, aggregateStream, aggregateID, startEventNumber)
	q.SetConsistency(gocql.LocalQuorum)

	ret := []RecordedEventData{}
	iter := q.Iter()
	defer q.Release()

	for {
		// New map each iteration
		row := make(map[string]interface{})
		if !iter.MapScan(row) {
			break
		}

		r := RecordedEventData{
			AggregateStream: row["aggregatestream"].(string),
			AggrehateID:     row["aggregateid"].(string),
			Version:         EventVersion(row["version"].(int64)),
			Created:         row["createtime"].(time.Time),
			EventData: EventData{
				ID:       row["eventid"].(string),
				Type:     row["eventtype"].(string),
				Data:     row["data"].([]byte),
				Metadata: row["metadata"].([]byte),
			},
		}
		ret = append(ret, r)
	}

	return ret, nil
}

func (es *cassandraEventStore) Close() error {
	es.session.Close()
	return nil
}
