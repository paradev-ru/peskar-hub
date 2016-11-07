package main

import (
	crand "crypto/rand"
	"fmt"
	"math"
	"math/big"
	"math/rand"
	"strings"
	"sync"
	"time"
)

var (
	once sync.Once

	SeededSecurely bool
)

func init() {
	SeedMathRand()
}

func SeedMathRand() {
	once.Do(func() {
		n, err := crand.Int(crand.Reader, big.NewInt(math.MaxInt64))
		if err != nil {
			rand.Seed(time.Now().UTC().UnixNano())
			return
		}
		rand.Seed(n.Int64())
		SeededSecurely = true
	})
}

func RandomUuid() (string, error) {
	var id string

	buf := make([]byte, 16)
	_, err := rand.Read(buf)
	if err != nil {
		return id, err
	}
	id = fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", buf[:4], buf[4:6], buf[6:8], buf[8:10], buf[10:])
	id = strings.ToUpper(id)
	return id, nil
}
