package warplib

import "sort"

type int64Slice []int64

func (x int64Slice) Len() int           { return len(x) }
func (x int64Slice) Less(i, j int) bool { return x[i] < x[j] }
func (x int64Slice) Swap(i, j int)      { x[i], x[j] = x[j], x[i] }

func sortInt64s(x []int64) { sort.Sort(int64Slice(x)) }
