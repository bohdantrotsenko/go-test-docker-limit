package main

import (
	"bytes"
	"fmt"
	"math/rand"
	"runtime"
	"sync/atomic"
	"time"
)

const bufSize = 1 * 1024 * 1024
const freeP = 0.499

const softLimit = 180 * 1024 * 1024

var degradedOperationsInterval = 2 * time.Second
var degradedOperationsFlag = int32(0)

func isDegraded() bool {
	return atomic.LoadInt32(&degradedOperationsFlag) > 0
}

type memStatsLight struct {
	Alloc         uint64
	TotalAlloc    uint64
	Sys           uint64
	Lookups       uint64
	Mallocs       uint64
	Frees         uint64
	HeapAlloc     uint64
	HeapSys       uint64
	HeapIdle      uint64
	HeapInuse     uint64
	HeapReleased  uint64
	HeapObjects   uint64
	StackInuse    uint64
	StackSys      uint64
	MSpanInuse    uint64
	MSpanSys      uint64
	MCacheInuse   uint64
	MCacheSys     uint64
	BuckHashSys   uint64
	GCSys         uint64
	OtherSys      uint64
	NextGC        uint64
	LastGC        uint64
	PauseTotalNs  uint64
	NumGC         uint32
	NumForcedGC   uint32
	GCCPUFraction float64
}

func copyMemStats(dst *memStatsLight, src *runtime.MemStats) {
	dst.Alloc = src.Alloc
	dst.TotalAlloc = src.TotalAlloc
	dst.Sys = src.Sys
	dst.Lookups = src.Lookups
	dst.Mallocs = src.Mallocs
	dst.Frees = src.Frees
	dst.HeapAlloc = src.HeapAlloc
	dst.HeapSys = src.HeapSys
	dst.HeapIdle = src.HeapIdle
	dst.HeapInuse = src.HeapInuse
	dst.HeapReleased = src.HeapReleased
	dst.HeapObjects = src.HeapObjects
	dst.StackInuse = src.StackInuse
	dst.StackSys = src.StackSys
	dst.MSpanInuse = src.MSpanInuse
	dst.MSpanSys = src.MSpanSys
	dst.MCacheInuse = src.MCacheInuse
	dst.MCacheSys = src.MCacheSys
	dst.BuckHashSys = src.BuckHashSys
	dst.GCSys = src.GCSys
	dst.OtherSys = src.OtherSys
	dst.NextGC = src.NextGC
	dst.LastGC = src.LastGC
	dst.PauseTotalNs = src.PauseTotalNs
	dst.NumGC = src.NumGC
	dst.NumForcedGC = src.NumForcedGC
	dst.GCCPUFraction = src.GCCPUFraction
}

var gcInfo chan struct{}
var gcDegraded chan struct{}

func finalizer(obj interface{}) {
	select {
	case gcInfo <- struct{}{}:
	default:
	}
	runtime.SetFinalizer(bytes.NewBuffer(make([]byte, 256)), finalizer)
}

func init() {
	gcInfo = make(chan struct{}, 1)
	gcDegraded = make(chan struct{}, 1)
	runtime.SetFinalizer(bytes.NewBuffer(make([]byte, 256)), finalizer)
	go func() {
		var stat runtime.MemStats
		var statCleaned memStatsLight
		for gcIdx := 0; true; gcIdx++ {
			_ = <-gcInfo
			runtime.ReadMemStats(&stat)
			copyMemStats(&statCleaned, &stat)

			/*	// could be used for debugging
				jsonData, err := json.MarshalIndent(&statCleaned, "", "  ")
				if err != nil {
					log.Fatalf("err marshalling: %s", err)
				}

				err = ioutil.WriteFile(fmt.Sprintf("/data/memstat_%05d.txt", gcIdx), jsonData, 0666)
				if err != nil {
					log.Fatalf("err writing stat: %s", err)
				}*/

			// fmt.Printf("\n%+v\n", &statCleaned)
			if statCleaned.NextGC > softLimit {
				atomic.StoreInt32(&degradedOperationsFlag, 1)
				gcDegraded <- struct{}{}
			} else if atomic.LoadInt32(&degradedOperationsFlag) > 0 {
				atomic.StoreInt32(&degradedOperationsFlag, 0)
				fmt.Println("   --- permit to proceed --- ")
			}
		}
	}()
	go func() {
		for {
			_ = <-gcDegraded
			fmt.Println("   --- degraded --- ")
			time.Sleep(degradedOperationsInterval)
			runtime.GC()
		}
	}()
}

func main() {
	var bufs [][]byte
	total := int64(0)
	workDone := int64(0)

	go func() {
		for {
			fmt.Printf("  Total: %dM (done: %d)    \n", atomic.LoadInt64(&total)/1024/1024, atomic.LoadInt64(&workDone))
			time.Sleep(time.Second)
		}
	}()

	rnd := rand.New(rand.NewSource(16))
	for allocIdx := 0; true; allocIdx++ {
		freeMemoryInstead := rnd.Float32() <= freeP

		if freeMemoryInstead && len(bufs) > 0 {
			whatToFree := rnd.Intn(len(bufs))
			bufs[whatToFree] = bufs[len(bufs)-1]
			bufs[len(bufs)-1] = nil
			bufs = bufs[:len(bufs)-1]
			atomic.AddInt64(&total, -bufSize)
			continue
		}

		if isDegraded() {
			continue
		}

		buffer := make([]byte, bufSize)
		for i := 0; i < bufSize; i++ {
			buffer[i] = 12
		}
		bufs = append(bufs, buffer)
		atomic.AddInt64(&total, bufSize)
		atomic.AddInt64(&workDone, 1)
	}
}
