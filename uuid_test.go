package ble

import (
	"bytes"
	"testing"
)

var forward = [][]byte{
	{1, 2, 3, 4, 5, 6},
	{12, 143, 231, 123, 87, 124, 209},
	{3, 43, 223, 12, 54},
}

var reverse = [][]byte{
	{6, 5, 4, 3, 2, 1},
	{209, 124, 87, 123, 231, 143, 12},
	{54, 12, 223, 43, 3},
}

func TestReverse(t *testing.T) {

	for i := 0; i < len(forward); i++ {
		r := Reverse(forward[i])
		if !bytes.Equal(r, reverse[i]) {
			t.Errorf("Error: %v in reverse should be %v, but is: %v", forward[i], reverse[i], r)
		}
	}
}
