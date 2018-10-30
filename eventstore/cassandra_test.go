package eventstore

import "testing"

func TestNewCassandraEventSource(t *testing.T) {
	es, err := NewCassandraEventStore([]string{"127.0.0.1"}, "testns")
	if err != nil {
		t.Error(err)
	}
	es.Close()
}

func TestSaveOptions(t *testing.T) {
	es, err := NewCassandraEventStore([]string{"127.0.0.1"}, "testns")
	if err != nil {
		t.Error(err)
	}
	defer es.Close()

	err = es.AppendToStream("aggA", "1", ExpectedVersionStreamExists, []EventData{
		EventData{Type: "aggA_evA", ID: "1", Data: []byte("aggA_evA - 1")},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	err = es.AppendToStream("aggA", "1", ExpectedVersionEmptyStream, []EventData{
		EventData{Type: "aggA_evA", ID: "1", Data: []byte("aggA_evA - 1")},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	err = es.AppendToStream("aggA", "1", ExpectedVersionNoStream, []EventData{
		EventData{Type: "aggA_evA", ID: "1", Data: []byte("aggA_evA - 1")},
	})
	if err != nil {
		t.Fatal(err)
	}
	err = es.AppendToStream("aggA", "2", ExpectedVersionNoStream, []EventData{
		EventData{Type: "aggA_evA", ID: "1", Data: []byte("aggA_evA - 1")},
		EventData{Type: "aggA_evB", ID: "2", Data: []byte("aggA_evB - 2")},
	})
	if err != nil {
		t.Fatal(err)
	}
	items, err := es.ReadEventStream("aggA", "1", 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(items)
	t.Fail()
}

func TestReadOptions(t *testing.T) {
	es, err := NewCassandraEventStore([]string{"127.0.0.1"}, "testns")
	if err != nil {
		t.Error(err)
	}
	defer es.Close()
	items, err := es.ReadEventStream("aggA", "2", StreamStart, 100)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(items)
	t.Fail()
}
