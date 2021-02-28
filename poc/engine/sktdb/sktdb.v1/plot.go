package sktdb_v1

import (
	"bufio"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"sync/atomic"

	"github.com/Sukhavati-Labs/go-miner/logging"
	"github.com/Sukhavati-Labs/go-miner/poc"
	"github.com/Sukhavati-Labs/go-miner/poc/engine/sktdb"
	"github.com/Sukhavati-Labs/go-miner/poc/pocutil"
	"github.com/Sukhavati-Labs/go-miner/pocec"
	"github.com/shirou/gopsutil/mem"
)

const (
	maxPrePlotMem = 4 * 1024 * poc.MiB // 4 GiB, max memory used when generating HashMapA
	maxPlotMem    = 4 * 1024 * poc.MiB // 4 GiB, max memory used when generating HashMapB
	minPrePlotMem = 256 * poc.MiB      // 256 MiB, min memory used when generating HashMapA
	minPlotMem    = 256 * poc.MiB      // 256 MiB, min memory used when generating HashMapB
	minMapABufMem = 64 * poc.MiB       // 64 MiB, min memory used for reading MapA buffer when generating HashMapB

	waitQuitPlotInterval = 1 << 20 // wait interval on quit plot signal
)

func makeAvailableMemory(cache *MemCache, requiredMem, maxMem, minMem uint64) error {
	if requiredMem > maxMem {
		requiredMem = maxMem
	}
	stat, err := mem.VirtualMemory()
	if err != nil {
		return err
	}
	if available := stat.Available; requiredMem > available {
		if available < minMem {
			return ErrMemoryNotEnough
		}
		requiredMem = (available / minMem) * minMem
	}
	cache.Update(requiredMem)
	return nil
}

func (hm *HashMapA) makeAvailableMemory(cache *MemCache, requiredMem uint64) error {
	return makeAvailableMemory(cache, requiredMem, maxPrePlotMem, minPrePlotMem)
}

func (hm *HashMapB) makeAvailableMemory(cache *MemCache, requiredMem uint64) error {
	return makeAvailableMemory(cache, requiredMem, maxPlotMem, minPlotMem)
}

func NewSktDBV1(rootPath string, ordinal int64, pubKey *pocec.PublicKey, bitLength int) (*SktDBV1, error) {
	mdb, err := OpenDB(rootPath, ordinal, pubKey, bitLength)
	if err != nil {
		if err != sktdb.ErrDBDoesNotExist {
			return nil, err
		}
		mdb, err = CreateDB(rootPath, ordinal, pubKey, bitLength)
		if err != nil {
			return nil, err
		}
	}

	return mdb.(*SktDBV1), nil
}

func (sdb *SktDBV1) executePlot(result chan error) {
	var err error
	var cache = NewMemCache(0)
	defer func() {
		cache.Release()
		if result != nil {
			result <- err
		}
		atomic.StoreInt32(&sdb.plotting, 0)
		sdb.wg.Done()
	}()

	logging.CPrint(logging.INFO, "start plotting",
		logging.LogFormat{"bit_length": sdb.bl, "pub_key": hex.EncodeToString(sdb.pubKey.SerializeCompressed())})
	if err = sdb.prePlotWork(cache); err != nil {
		if err == ErrStopPlotting {
			err = nil
			return
		}
		logging.CPrint(logging.ERROR, "pre plot fail",
			logging.LogFormat{"bit_length": sdb.bl, "pub_key": hex.EncodeToString(sdb.pubKey.SerializeCompressed()), "err": err})
		return
	}
	if err = sdb.plotWork(cache); err != nil {
		if err == ErrStopPlotting {
			err = nil
			return
		}
		logging.CPrint(logging.ERROR, "plot fail",
			logging.LogFormat{"bit_length": sdb.bl, "pub_key": hex.EncodeToString(sdb.pubKey.SerializeCompressed()), "err": err})
		return
	}
	logging.CPrint(logging.INFO, "remove hashMapA",
		logging.LogFormat{"bit_length": sdb.bl, "pub_key": hex.EncodeToString(sdb.pubKey.SerializeCompressed())})
	sdb.HashMapA.Close()
	os.Remove(sdb.filePathA)
	sdb.HashMapA = nil
	logging.CPrint(logging.INFO, "plot finished",
		logging.LogFormat{"bit_length": sdb.bl, "pub_key": hex.EncodeToString(sdb.pubKey.SerializeCompressed())})
}

func (sdb *SktDBV1) prePlotWork(cache *MemCache) error {
	var hmA = sdb.HashMapA
	var recordSize = hmA.recordSize
	var bl = hmA.bitLength
	var pkHash = hmA.pkHash
	var b8 [8]byte
	var half = hmA.volume / 2

	var logCheckpointInterval = hmA.volume / 50
	var checkpoint = hmA.ReadCheckpoint()
	logging.CPrint(logging.INFO, fmt.Sprintf("load checkpoint for HashMapA: %d/%d (%d/%d)", checkpoint, hmA.volume, checkpoint/logCheckpointInterval, 50),
		logging.LogFormat{"bit_length": sdb.bl, "pub_key": hex.EncodeToString(sdb.pubKey.SerializeCompressed())})

	var ensureCacheMemory = func(startPoint pocutil.PoCValue) error {
		return hmA.makeAvailableMemory(cache, uint64(hmA.volume-startPoint)*uint64(recordSize))
	}
	var calcWindowSize = func() pocutil.PoCValue {
		rem := (cache.Len() / recordSize) & 1
		return pocutil.PoCValue(cache.Len()/recordSize - rem)
	}
	for startPoint := checkpoint; startPoint < hmA.volume; {
		if err := ensureCacheMemory(startPoint); err != nil {
			return err
		}
		endPoint := startPoint + calcWindowSize() // slide windows defined by [start, end)
		logging.CPrint(logging.DEBUG, "assign hashMapA calculation work", logging.LogFormat{"start_point": startPoint, "end_point": endPoint})
		for x := pocutil.PoCValue(0); x < hmA.volume; x++ {
			// calc and write the cache
			y := pocutil.P(x, bl, pkHash)
			if y < half {
				y = y * 2
			} else {
				y = pocutil.FlipValue(y, bl)*2 + 1
			}
			// write data within the window defined by [start, end)
			if startPoint <= y && y < endPoint {
				target := int(y-startPoint) * recordSize
				binary.LittleEndian.PutUint64(b8[:], uint64(x))
				cache.WriteAt(b8[:recordSize], int64(target))
			}

			// log and respond to quit signal
			if x%waitQuitPlotInterval == 0 {
				select {
				case <-sdb.stopPlotCh:
					logging.CPrint(logging.INFO, "pre plot aborted",
						logging.LogFormat{"bit_length": sdb.bl, "pub_key": hex.EncodeToString(sdb.pubKey.SerializeCompressed())})
					return ErrStopPlotting
				default:
				}
			}
			if x%logCheckpointInterval == 0 {
				totalProgress := float64(startPoint)/float64(hmA.volume) + float64(endPoint-startPoint)/float64(hmA.volume)*(float64(x/logCheckpointInterval)/50)
				logging.CPrint(logging.DEBUG, fmt.Sprintf("current round %d/%d (%d), total progress %f", x/logCheckpointInterval, 50, x, totalProgress))
			}
		}
		if n, err := cache.WriteToWriter(sdb.stopPlotCh, hmA.data, 0, int64(hmA.offset)+int64(startPoint)*int64(recordSize), int64(cache.Len())); err != nil {
			logging.CPrint(logging.ERROR, "fail on writing cache to file", logging.LogFormat{"err": err, "n": n})
			return err
		}
		hmA.data.Sync() // write pre-plot data first

		hmA.checkpoint = startPoint + 1
		hmA.UpdateCheckpoint()
		hmA.data.Sync() // then write new checkpoint
		startPoint = endPoint
	}

	hmA.checkpoint = hmA.volume
	hmA.UpdateCheckpoint()
	hmA.data.Sync()
	return nil
}

func (sdb *SktDBV1) plotWork(cache *MemCache) error {
	var hmA, hmB = sdb.HashMapA, sdb.HashMapB
	var half = hmB.volume / 2
	var bl = hmA.bitLength
	var pkHash = hmA.pkHash
	var recordSize = pocutil.RecordSize(bl)
	var bs = make([]byte, recordSize*2)

	var logCheckpointInterval = hmB.volume / (50 * 2)
	var checkpoint = hmB.ReadCheckpoint()
	logging.CPrint(logging.INFO, fmt.Sprintf("load checkpoint for HashMapB: %d/%d (%d/%d)", checkpoint*2, hmB.volume, checkpoint/logCheckpointInterval, 50),
		logging.LogFormat{"bit_length": sdb.bl, "pub_key": hex.EncodeToString(sdb.pubKey.SerializeCompressed())})

	var bytesEqualZero = func(bs []byte) bool {
		for i := 0; i < recordSize; i++ {
			if bs[i] != 0 {
				return false
			}
		}
		return true
	}
	var ensureCacheMemory = func(startPoint pocutil.PoCValue) error {
		return hmB.makeAvailableMemory(cache, uint64(half-startPoint)*uint64(recordSize)<<2)
	}
	var calcWindowSize = func() pocutil.PoCValue {
		return pocutil.PoCValue((cache.Len() / recordSize) >> 2)
	}
	for startPoint := checkpoint; startPoint < half; {
		if err := ensureCacheMemory(startPoint); err != nil {
			return err
		}
		endPoint := startPoint + calcWindowSize() // slide windows defined by [start, end)
		doubleStartPoint, doubleEndPoint := startPoint<<1, endPoint<<1
		logging.CPrint(logging.DEBUG, "assign hashMapB calculation work", logging.LogFormat{"double_start_point": doubleStartPoint, "double_end_point": doubleEndPoint})

		if _, err := hmA.data.Seek(int64(hmA.offset), 0); err != nil {
			return err
		}
		bufRdA := bufio.NewReaderSize(hmA.data, minMapABufMem)

		for y := pocutil.PoCValue(0); y < half; y++ {
			bufRdA.Read(bs)
			x, xp := bs[:recordSize], bs[recordSize:]
			if !bytesEqualZero(x) && !bytesEqualZero(xp) {
				z := pocutil.FB(x, xp, bl, pkHash)
				if doubleStartPoint <= z && z < doubleEndPoint {
					target := int(z-doubleStartPoint) * recordSize * 2
					cache.WriteAt(x, int64(target))
					cache.WriteAt(xp, int64(target+recordSize))
				}
				zp := pocutil.FB(xp, x, bl, pkHash)
				if doubleStartPoint <= zp && zp < doubleEndPoint {
					target := int(zp-doubleStartPoint) * recordSize * 2
					cache.WriteAt(xp, int64(target))
					cache.WriteAt(x, int64(target+recordSize))
				}
			}

			if y%waitQuitPlotInterval == 0 {
				select {
				case <-sdb.stopPlotCh:
					logging.CPrint(logging.INFO, "plot aborted",
						logging.LogFormat{"bit_length": sdb.bl, "pub_key": hex.EncodeToString(sdb.pubKey.SerializeCompressed())})
					return ErrStopPlotting
				default:
				}
			}
			if y%logCheckpointInterval == 0 {
				totalProgress := float64(startPoint*2)/float64(hmA.volume) + float64(endPoint-startPoint)*2.0/float64(hmA.volume)*(float64(y/logCheckpointInterval)/50)
				logging.CPrint(logging.DEBUG, fmt.Sprintf("current round %d/%d (%d), total progress %f", y/logCheckpointInterval, 50, y, totalProgress))
			}
		}
		if n, err := cache.WriteToWriter(sdb.stopPlotCh, hmB.data, 0, int64(hmB.offset)+int64(startPoint)*int64(recordSize)*4, int64(cache.Len())); err != nil {
			logging.CPrint(logging.ERROR, "fail on writing cache to file", logging.LogFormat{"err": err, "n": n})
			return err
		}
		hmB.data.Sync() // write plot data first

		hmB.checkpoint = startPoint + 1
		hmB.UpdateCheckpoint()
		hmB.data.Sync() // then update checkpoint
		startPoint = endPoint
	}

	hmB.checkpoint = half
	hmB.UpdateCheckpoint()
	hmB.data.Sync()
	return nil
}
