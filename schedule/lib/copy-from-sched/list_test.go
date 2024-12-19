package sched

import (
	"reflect"
	"testing"
	"time"
)

func TestListPushBack(t *testing.T) {
	var (
		a taskList
		b taskList
	)
	a = a.PushBack(newTaskID(0))
	a = a.PushBack(newTaskID(1))

	b = a.PushBack(newTaskID(2))
	b = b.PushBack(newTaskID(3))
	// Expect that B contains the same list as A, but extended.
	// That is, taskList acts like a Go slices.
	if b.head != a.head {
		t.Errorf("B list was copied")
	}

	expectListTasks(t, a, []taskID{0, 1})
	expectListTasks(t, b, []taskID{0, 1, 2, 3})

	// Now append an element to A list. Note that before this push A and B were
	// the same lists with just different bounds. A's tail contains pointer to
	// the B's next elements, thus now we can not simply "append" new element
	// to the list – A's left part must be deep copied.
	a = a.PushBack(newTaskID(4))
	if a.head == b.head {
		t.Errorf("A was not copied")
	}
	expectListTasks(t, a, []taskID{0, 1, 4})
	expectListTasks(t, b, []taskID{0, 1, 2, 3})
}

func TestListClone(t *testing.T) {
	var a taskList
	a = a.PushBack(newTaskID(0))
	a = a.PushBack(newTaskID(1))
	a = a.PushBack(newTaskID(2))

	// PushBack another value such that A's tail element has non-nil next
	// field.
	a.PushBack(newTaskID(42))

	c := a.clone()
	expectListTasks(t, c, []taskID{0, 1, 2})
	if c.head == a.head || c.tail == a.tail {
		t.Errorf("list was not copied")
	}
	if c.tail.next != nil {
		t.Errorf("copied list's tail has non-nil next field")
	}
}

func TestListCloneSingle(t *testing.T) {
	var a taskList
	a = a.PushBack(newTaskID(0))
	c := a.clone()
	expectListTasks(t, c, []taskID{0})
	if c.head == a.head || c.tail == a.tail {
		t.Errorf("list was not copied")
	}
}

func TestListRemoveHead(t *testing.T) {
	t0 := newTaskID(0)

	var a taskList
	a = a.PushBack(t0)
	a = a.PushBack(newTaskID(1))
	a = a.PushBack(newTaskID(2))

	b := a.Remove(t0)
	expectListTasks(t, b, []taskID{1, 2})
	if b.head != a.head.next {
		t.Errorf("unexpected list head: %v; want %v", b.head, a.head.next)
	}
	if b.tail != a.tail {
		t.Errorf("unexpected list tail: %v; want %v", b.tail, a.tail)
	}
}

func TestListRemoveMiddle(t *testing.T) {
	t1 := newTaskID(1)
	var a taskList
	a = a.PushBack(newTaskID(0))
	a = a.PushBack(t1)
	a = a.PushBack(newTaskID(2))

	b := a.Remove(t1)
	expectListTasks(t, b, []taskID{0, 2})
	if b.head == a.head {
		t.Errorf("list head was not copied")
	}
	if b.tail != a.tail {
		t.Errorf("unexpected list tail: %v; want %v", b.tail, a.tail)
	}
}

func TestListRemoveTail(t *testing.T) {
	t2 := newTaskID(2)
	var a taskList
	a = a.PushBack(newTaskID(0))
	a = a.PushBack(newTaskID(1))
	a = a.PushBack(t2)

	b := a.Remove(t2)
	expectListTasks(t, b, []taskID{0, 1})
	if b.head != a.head {
		t.Errorf("list head was copied")
	}
	if b.head.next != a.head.next {
		t.Errorf("list middle was copied")
	}
}

func TestListRemoveNothing(t *testing.T) {
	var a, b taskList
	a = a.PushBack(newTaskID(0))
	a = a.PushBack(newTaskID(1))
	a = a.PushBack(newTaskID(2))

	t3 := newTaskID(3)
	b.PushBack(t3)
	c := a.Remove(t3)

	expectListTasks(t, a, []taskID{0, 1, 2})
	if c.head != a.head {
		t.Errorf("list head was changed")
	}
	if c.head.next != a.head.next {
		t.Errorf("list middle was changed")
	}
	if c.tail != a.tail {
		t.Errorf("list head was chanded")
	}
	if a.tail.next != nil {
		t.Errorf("list tail non-nil next field")
	}
}

func TestListRemoveTwice(t *testing.T) {
	t1 := newTaskID(1)
	t2 := newTaskID(2)
	var a taskList
	a = a.PushBack(newTaskID(0))
	a = a.PushBack(t1)
	a = a.PushBack(t2)
	a = a.PushBack(newTaskID(3))

	// Remove t2 element to make left part of the list to be copied.
	a = a.Remove(t2)
	expectListTasks(t, a, []taskID{0, 1, 3})

	// Now remote t1 to ensure that even after copying t1 points to the same
	// element.
	a = a.Remove(t1)
	expectListTasks(t, a, []taskID{0, 3})
}

func TestTaskSlicePushBack(t *testing.T) {
	s := newTaskSlice()
	tasksMap := make(map[taskID]*task)

	for _, tc := range []struct {
		name     string
		push     []taskID
		remove   []taskID
		expected []taskID
	}{
		{
			name:     "push-0",
			push:     []taskID{0},
			expected: []taskID{0},
		},
		{
			name:     "push-1-2",
			push:     []taskID{1, 2},
			expected: []taskID{0, 1, 2},
		},
		{
			name:     "remove-1",
			remove:   []taskID{1},
			expected: []taskID{0, 2},
		},
		{
			name:     "remove-2",
			remove:   []taskID{2},
			expected: []taskID{0},
		},
		{
			name:     "remove-invalid-task",
			remove:   []taskID{3},
			expected: []taskID{0},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, tID := range tc.push {
				tsk, has := tasksMap[tID]
				if !has {
					tsk = newTaskID(int(tID))
					tasksMap[tID] = tsk
				}
				s.PushBack(tsk)
			}
			for _, tID := range tc.remove {
				tsk, has := tasksMap[tID]
				if !has {
					tsk = newTaskID(int(tID))
					tasksMap[tID] = tsk
				}
				s.Remove(tsk)
			}
			expectTaskSliceTasks(t, s, tc.expected)
		})
	}
}

type taskID int

// Exec implements Task interface.
func (t taskID) Exec(time.Time) {}

func newTaskID(id int) *task {
	return newTask(taskID(id))
}

func expectListTasks(t testing.TB, l taskList, exp []taskID) {
	n := l.Len()
	if m := len(exp); n != m {
		t.Errorf("unexpected list size: %d; want %d", n, m)
		return
	}
	act := make([]taskID, 0, n)
	l.ForEach(func(t *task) {
		act = append(act, t.Task.(taskID))
	})
	if !reflect.DeepEqual(act, exp) {
		t.Errorf("unexpected list tasks: %v; want %v", act, exp)
	}
}

func expectTaskSliceTasks(t testing.TB, s *taskSlice, exp []taskID) {
	n := s.Len()
	if m := len(exp); n != m {
		t.Errorf("unexpected list size: %d; want %d", n, m)
		return
	}
	act := make([]taskID, 0, n)
	s.ForEach(func(t *task) {
		act = append(act, t.Task.(taskID))
	})
	if !reflect.DeepEqual(act, exp) {
		t.Errorf("unexpected list tasks: %v; want %v", act, exp)
	}
}
