package main

import "encoding/json"

type rowCoverage struct {
	Rows          int64      `json:"rows"`
	NonNull       [50]int64  `json:"non_null"`
	Null          [50]int64  `json:"null"`
	Empty         [50]int64  `json:"empty_text"`
	Zero          [50]int64  `json:"numeric_zero"`
	ToNull        [50]int64  `json:"to_null"`
	FromNull      [50]int64  `json:"from_null"`
	JSONBytes     int64      `json:"row_json_bytes"`
	ByteHistogram [128]int64 `json:"row_json_byte_histogram_128_byte_buckets"`
}

func (c *rowCoverage) row(values []any) {
	c.Rows++
	for i, v := range values {
		if v == nil {
			c.Null[i]++
			continue
		}
		c.NonNull[i]++
		switch n := v.(type) {
		case string:
			if n == "" {
				c.Empty[i]++
			}
		case int64:
			if n == 0 {
				c.Zero[i]++
			}
		}
	}
	b, _ := json.Marshal(values)
	c.JSONBytes += int64(len(b))
	bucket := len(b) / 128
	if bucket >= len(c.ByteHistogram) {
		bucket = len(c.ByteHistogram) - 1
	}
	c.ByteHistogram[bucket]++
}

func (c *rowCoverage) change(index int, before, after any) {
	if before == nil && after != nil {
		c.FromNull[index]++
	}
	if before != nil && after == nil {
		c.ToNull[index]++
	}
}
