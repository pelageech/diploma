package simple_time_effective

import (
	"github.com/pelageech/diploma/stand"
)

type Node struct {
	key int
	val *stand.Job
	pos int
}

type JobsHeap []*Node

func (h JobsHeap) Len() int { return len(h) }
func (h JobsHeap) Less(i, j int) bool {
	return h[i].key > h[j].key
}
func (h JobsHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].pos, h[j].pos = i, j
}
func (h *JobsHeap) Push(x interface{}) {
	n := x.(*Node)
	n.pos = len(*h)
	*h = append(*h, n)
}
func (h *JobsHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}
