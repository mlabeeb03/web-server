package main

import "sync/atomic"

type apiConfig struct {
	fileserverHits atomic.Int32
}
