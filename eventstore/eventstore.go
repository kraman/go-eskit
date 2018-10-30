package eventstore

import "time"

//EventVersion is the version of an event being inserted or queried from the event store
type EventVersion int64

const (
	//StreamStart can be used to query events from the start of the stream
	StreamStart EventVersion = -1

	//ExpectedVersionAny is equavalent of ExpectedVersionNoStream || ExpectedVersionStreamExists
	ExpectedVersionAny EventVersion = -3

	//ExpectedVersionNoStream requires a new stream when saving events
	ExpectedVersionNoStream EventVersion = -1

	//ExpectedVersionEmptyStream requires an empty stream when saving events
	ExpectedVersionEmptyStream EventVersion = 0

	//ExpectedVersionStreamExists only requires that a stream is present when saving events
	ExpectedVersionStreamExists EventVersion = -2
)

//EventData represents the event to be saved
type EventData struct {
	ID       string
	Type     string
	Data     []byte
	Metadata []byte
}

//RecordedEventData represents a saved event from the store
type RecordedEventData struct {
	EventData
	AggregateStream string
	AggrehateID     string
	Version         EventVersion
	Created         time.Time
}

//RecordedEvents is a list of events from the store
type RecordedEvents []RecordedEventData

//EventStore represents a persisted store of events
type EventStore interface {
	AppendToStream(aggregateStream string, aggregateID string, expectedVersion EventVersion, events []EventData) error
	ReadEventStream(aggregateStream string, aggregateID string, startEventNumber EventVersion, len uint32) (RecordedEvents, error)
	Close() error
}
