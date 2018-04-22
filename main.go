package main

import (
	"bytes"
	"fmt"
	"math/rand"
	"runtime"
	"sync/atomic"
	"time"
)

///////////////////////////////////////
// work-load

const perWorkItemAllocation = 1 * 1024 * 1024
const workItemMinDuration = time.Millisecond * 20
const workItemMaxDuration = time.Millisecond * 1200
const workItemAppearanceInterval = time.Millisecond // 1000 times/seq * 0.5sec/req avg => 250MB avg

func workItem() {
	buffer := make([]byte, perWorkItemAllocation)
	for i := 0; i < perWorkItemAllocation; i++ {
		buffer[i] = 12 // this is to overcome linux's memory overcommit
	}
	time.Sleep(time.Duration(rand.Int63n(int64(workItemMaxDuration-workItemMinDuration)+1)) + workItemMinDuration)

	// becase go 1.10 is smart enough to release the buffer if it's not access at this point
	max := byte(0)
	for i := 0; i < perWorkItemAllocation; i++ {
		if max < buffer[i] {
			max = buffer[i]
		}
	}
}

///////////////////////////////////////
// GC-watcher related settings

// softLimit
const softLimit = 280 * 1024 * 1024

var degradedOperationsInterval = 2 * time.Second
var degradedOperationsFlag = int32(0)

func isDegraded() bool {
	return atomic.LoadInt32(&degradedOperationsFlag) > 0
}

///////////////////////////////////////
// GC-watcher 'internals'

// memStatsLight is a copy of runtime.MemStats omitting some fields
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
	runtime.SetFinalizer(bytes.NewBuffer(make([]byte, 256)), finalizer) // a trick to watch for GC
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

			// fmt.Printf("\n%+v\n", &statCleaned) // more statistics

			// fmt.Printf("NextGC: %d\n\n", statCleaned.NextGC/1024/1024) // less statistics
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

///////////////////////////////////////

func main() {
	rand.Seed(time.Now().UnixNano())

	total := int64(0)
	workDone := int64(0)

	go func() {
		for {
			fmt.Printf("  Total: %dM (done: %d)    \n", atomic.LoadInt64(&total)/1024/1024, atomic.LoadInt64(&workDone))
			time.Sleep(time.Second / 4)
		}
	}()

	for allocIdx := 0; true; allocIdx++ {
		time.Sleep(workItemAppearanceInterval)

		if isDegraded() {
			continue
		}

		atomic.AddInt64(&total, perWorkItemAllocation)
		go func() {
			workItem()
			atomic.AddInt64(&total, -perWorkItemAllocation)
			atomic.AddInt64(&workDone, 1)
		}()
	}
}
