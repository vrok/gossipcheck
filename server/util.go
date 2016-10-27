package server

import "math/rand"

const lettersCnt = 'z' - 'a'

// randStr generates a random string of length n.
func randStr(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte('a' + rand.Int()%lettersCnt)
	}
	return string(b)
}
