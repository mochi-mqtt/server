package ring

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewBuffer(t *testing.T) {
	var size int64 = 256
	buf := NewBuffer(size)

	require.NotNil(t, buf.buffer)
	require.Equal(t, size, len(buf.buffer))
	require.Equal(t, size, buf.size)
}

/*
func TestReadFromIO(t *testing.T) {
	buf := NewBuffer(128)
	in := bytes.Repeat([]byte{'-'}, 32)
	n, err := buf.Read(bytes.NewReader(in))
	require.Equal(t, 3, n)
	require.Equal(t, err, io.EOF)
}
*/

func TestAwaitCapacity(t *testing.T) {
	buf := NewBuffer(16)
	buf.tail, buf.head = 0, 0 // OK 4, 0
	buf.awaitCapacity(4)

	buf.tail, buf.head = 6, 6 // OK 6, 10
	buf.awaitCapacity(4)

	buf.tail, buf.head = 12, 16 // OK 16, 4
	buf.awaitCapacity(4)

	buf.tail, buf.head = 16, 12 // OK 16, 16
	buf.awaitCapacity(4)

	// Plenty of room for next block
	buf.tail, buf.head = 3, 6 // OK 3, 10
	buf.awaitCapacity(4)

	fmt.Println("---")

	// head + n > tail wait will wrap and overrun the tail! 4...4
	buf.tail, buf.head = 1, 16
	buf.awaitCapacity(4)

	// Tail is greater than head - we've wrapped and caught up to the tail. 9...9
	buf.tail, buf.head = 7, 5
	buf.awaitCapacity(4)

}
