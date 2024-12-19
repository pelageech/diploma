package sched

import (
	"sync"
	"time"
)

type task struct {
	Task
	mu       sync.Mutex
	disabled bool
}

func newTask(t Task) *task {
	return &task{
		Task: t,
	}
}

func (t *task) exec(now time.Time) {
	t.mu.Lock()
	if !t.disabled {
		t.Task.Exec(now)
	}
	t.mu.Unlock()
}

func (t *task) disable() {
	t.mu.Lock()
	t.disabled = true
	t.mu.Unlock()
}

type element struct {
	task *task
	next *element
}

// taskList is an immutable linked list of tasks.
//
// TODO: It can be optimized to track its readers and make mutations inplace
// when there are no readers.
type taskList struct {
	head *element
	// NOTE: tail's next field is actually mutable in pushBack() so do not read
	// it concurrently.
	tail *element
	len  int
}

// PushBack inserts a new element with task t at the back of the new list.
// It returns next list generation and created element.
func (l taskList) PushBack(x *task) taskList {
	e := &element{task: x}
	return l.pushBack(e)
}

// Remove returns list without task x.
func (l taskList) Remove(x *task) taskList {
	for i, p, e := 0, (*element)(nil), l.head; e != nil; i, p, e = i+1, e, e.next {
		if e.task == x {
			var a, b taskList
			if p != nil {
				a = taskList{
					head: l.head,
					tail: p,
					len:  i,
				}
			}
			if n := e.next; n != nil {
				b = taskList{
					head: n,
					tail: l.tail,
					len:  l.len - i - 1,
				}
			}
			return join(a, b)
		}
	}
	return l
}

// Len returns the number of elements of the list.
func (l *taskList) Len() int {
	return l.len
}

// ForEach calls f with value of every element of the list.
func (l *taskList) ForEach(f func(*task)) {
	for e := l.head; e != nil; {
		f(e.task)
		if e == l.tail {
			// NOTE: do not touch l.tail.next to prevent races. That is, in
			// pushBack() call there is an optimization to modify tail's next
			// field if it is empty to get rid of list copying.
			break
		}
		e = e.next
	}
}

func (l taskList) pushBack(e *element) taskList {
	if l.head == nil {
		l.head = e
	} else {
		if l.tail.next != nil {
			l = l.clone()
		}
		l.tail.next = e
	}
	l.tail = e
	l.len++
	return l
}

func (l taskList) clone() taskList {
	n := l.Len()
	ts := make([]element, n)
	var p *element
	for i, e := 0, l.head; e != nil && i < n; i, p, e = i+1, e, e.next {
		ts[i] = *e
		if e == l.tail {
			ts[i].next = nil
		}
		e = &ts[i]
		if p != nil {
			p.next = e
		} else {
			l.head = e
		}
	}
	l.tail = p
	return l
}

func join(a, b taskList) taskList {
	if b.head == nil {
		return a
	}
	if a.head == nil {
		return b
	}
	c := a.pushBack(b.head)
	c.len = a.len + b.len
	return c
}

type taskSlice struct {
	lock    sync.Mutex
	tasks   []*task
	taskMap map[*task]int
}

func newTaskSlice() *taskSlice {
	return &taskSlice{
		tasks:   make([]*task, 0, 16),
		taskMap: make(map[*task]int, 16),
	}
}

func (t *taskSlice) PushBack(x *task) *taskSlice {
	t.lock.Lock()
	t.tasks = append(t.tasks, x)
	t.taskMap[x] = len(t.tasks) - 1
	t.lock.Unlock()
	return t
}

func (t *taskSlice) Remove(x *task) *taskSlice {
	t.lock.Lock()
	defer t.lock.Unlock()

	idx, has := t.taskMap[x]
	if !has {
		return t
	}

	if len(t.tasks) == idx+1 {
		t.tasks = t.tasks[:len(t.tasks)-1]
		delete(t.taskMap, x)
		return t
	}

	lastTask := t.tasks[len(t.tasks)-1]
	t.tasks[idx] = lastTask
	t.tasks = t.tasks[:len(t.tasks)-1]
	delete(t.taskMap, x)
	t.taskMap[lastTask] = idx

	return t
}

func (t *taskSlice) Len() int {
	t.lock.Lock()
	ret := len(t.tasks)
	t.lock.Unlock()
	return ret
}

func (t *taskSlice) ForEach(f func(*task)) {
	t.lock.Lock()
	tasks := append([]*task(nil), t.tasks...)
	t.lock.Unlock()
	for _, t := range tasks {
		f(t)
	}
}
