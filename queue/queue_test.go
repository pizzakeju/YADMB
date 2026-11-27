package queue

import "testing"

func TestQueue_NewQueue(t *testing.T) {
	q := NewQueue()

	if q.Queue == nil {
		t.Error("Expected a new queue, got nil")
	}
}

func TestQueue_IsEmpty(t *testing.T) {
	q := NewQueue()

	if !q.IsEmpty() {
		t.Error("Expected an empty queue, got a non-empty queue")
	}
}

func TestQueue_AddElements(t *testing.T) {
	q := NewQueue()

	q.AddElements(Element{ID: "1"}, Element{ID: "2"}, Element{ID: "3"})

	if q.IsEmpty() {
		t.Error("Expected a non-empty queue, got an empty queue")
	}

	if len(q.Queue) != 3 {
		t.Error("Expected 3, got", len(q.Queue))
	}
}

func TestQueue_GetFirstElement(t *testing.T) {
	q := NewQueue()

	if q.GetFirstElement() != nil {
		t.Error("Expected nil, got an element")
	}

	q.AddElements(Element{ID: "1"}, Element{ID: "2"}, Element{ID: "3"})

	if q.GetFirstElement().ID != "1" {
		t.Error("Expected 1, got", q.GetFirstElement().ID)
	}
}

func TestQueue_AddElementsPriority(t *testing.T) {
	q := NewQueue()

	q.AddElements(Element{ID: "1"}, Element{ID: "2"}, Element{ID: "3"})

	q.AddElementsPriority(Element{ID: "4"})

	if q.GetFirstElement().ID == "4" {
		t.Error("Expected 4, got", q.GetFirstElement().ID)
	}
}

func TestQueue_RemoveFirstElement(t *testing.T) {
	q := NewQueue()

	q.AddElements(Element{ID: "1"}, Element{ID: "2"}, Element{ID: "3"})

	q.RemoveFirstElement()

	if q.GetFirstElement().ID != "2" {
		t.Error("Expected 2, got", q.GetFirstElement().ID)
	}
}

func TestElement_IsDownloading(t *testing.T) {
	if (Element{}).IsDownloading() {
		t.Error("Expected a cached element not to be downloading")
	}

	done := make(chan struct{})
	el := Element{Downloading: true, DownloadDone: done}
	if !el.IsDownloading() {
		t.Error("Expected the element to still be downloading")
	}

	close(done)
	if el.IsDownloading() {
		t.Error("Expected the download to be finished")
	}
}

func TestQueue_Move(t *testing.T) {
	q := NewQueue()
	q.AddElements(Element{ID: "1"}, Element{ID: "2"}, Element{ID: "3"}, Element{ID: "4"})

	if err := q.Move(1, 3); err != nil {
		t.Fatalf("Move returned error: %v", err)
	}

	ids := q.GetAllQueue()
	if ids[1].ID != "3" || ids[2].ID != "4" || ids[3].ID != "2" {
		t.Fatalf("unexpected order after move: %v", ids)
	}

	if err := q.Move(0, 9); err == nil {
		t.Fatal("expected error for out-of-range move")
	}
}

func TestQueue_RemoveAt(t *testing.T) {
	q := NewQueue()
	q.AddElements(Element{ID: "1"}, Element{ID: "2"}, Element{ID: "3"})

	if err := q.RemoveAt(1); err != nil {
		t.Fatalf("RemoveAt returned error: %v", err)
	}

	if q.GetLength() != 2 || q.GetAllQueue()[1].ID != "3" {
		t.Fatalf("unexpected queue after remove: %v", q.GetAllQueue())
	}

	if err := q.RemoveAt(5); err == nil {
		t.Fatal("expected error for out-of-range remove")
	}
}

func TestQueue_ShuffleRemaining(t *testing.T) {
	q := NewQueue()
	q.AddElements(Element{ID: "1"}, Element{ID: "2"}, Element{ID: "3"}, Element{ID: "4"})

	q.ShuffleRemaining()

	ids := q.GetAllQueue()
	if ids[0].ID != "1" {
		t.Fatalf("current song must stay first, got %v", ids)
	}
	if q.GetLength() != 4 {
		t.Fatalf("unexpected length after shuffle: %d", q.GetLength())
	}
}
