package warplib

import "strings"

type ContentLength int64

func (c ContentLength) v() int64 {
	return int64(c)
}

func (c ContentLength) vshort() int {
	return int(c)
}

func (c ContentLength) String() string {
	return c.Format(
		" ",
		SizeOptionTB,
		SizeOptionGB,
		SizeOptionMB,
		SizeOptionKB,
		SizeOptionBy,
	)
}

func (c ContentLength) Format(sep string, sizeOpts ...SizeOption) string {
	b := strings.Builder{}
	n := len(sizeOpts) - 1
	for i, opt := range sizeOpts {
		siz, rem := opt.Get(c)
		c = ContentLength(rem)
		if siz == 0 {
			continue
		}
		fl := opt.StringFrom(siz)
		b.WriteString(fl)
		if i == n {
			break
		}
		b.WriteString(sep)
	}
	return b.String()
}

func (c *ContentLength) IsUnknown() bool {
	return c.v() == -1
}
