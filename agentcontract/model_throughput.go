package agentcontract

import (
	"sort"
	"sync"
	"time"
)

type ModelThroughput struct {
	CostPerCall time.Duration
	SampleCount int
}

func supportedFloorCostPerCall() time.Duration {
	generating := time.Duration(float64(measuredOutputTokensPerModelCall) / supportedFloorOutputTokensPerSecond * float64(time.Second))
	return localCostPerModelCall + generating
}

func (throughput ModelThroughput) MeetsSupportedFloor() bool {
	return throughput.CostPerCall <= 0 || throughput.CostPerCall <= supportedFloorCostPerCall()
}

func DurationForIterationCount(iterationCount int, throughput ModelThroughput) time.Duration {
	floor := time.Duration(iterationCount) * supportedFloorCostPerCall() * durationMargin
	if throughput.CostPerCall <= 0 {
		return floor
	}
	return min(time.Duration(iterationCount)*throughput.CostPerCall*durationMargin, floor)
}

type ThroughputObserver struct {
	mutex           sync.Mutex
	latencyByModel  map[string][]time.Duration
	lastRecordedFor string
}

const throughputSampleCeiling = 200

func NewThroughputObserver() *ThroughputObserver {
	return &ThroughputObserver{latencyByModel: map[string][]time.Duration{}}
}

func (observer *ThroughputObserver) Record(modelName string, latency time.Duration, completionTokens int64) {
	if observer == nil || modelName == "" || latency <= 0 || completionTokens <= 0 {
		return
	}
	observer.mutex.Lock()
	defer observer.mutex.Unlock()
	latencies := append(observer.latencyByModel[modelName], latency)
	if len(latencies) > throughputSampleCeiling {
		latencies = latencies[len(latencies)-throughputSampleCeiling:]
	}
	observer.latencyByModel[modelName] = latencies
	observer.lastRecordedFor = modelName
}

func (observer *ThroughputObserver) ThroughputOfModelInUse() ModelThroughput {
	if observer == nil {
		return ModelThroughput{}
	}
	observer.mutex.Lock()
	modelName := observer.lastRecordedFor
	observer.mutex.Unlock()
	return observer.Throughput(modelName)
}

func (observer *ThroughputObserver) Throughput(modelName string) ModelThroughput {
	if observer == nil {
		return ModelThroughput{}
	}
	observer.mutex.Lock()
	defer observer.mutex.Unlock()
	latencies := observer.latencyByModel[modelName]
	if len(latencies) == 0 {
		return ModelThroughput{}
	}
	return ModelThroughput{CostPerCall: medianDuration(latencies), SampleCount: len(latencies)}
}

func medianDuration(values []time.Duration) time.Duration {
	ordered := append([]time.Duration{}, values...)
	sort.Slice(ordered, func(left int, right int) bool { return ordered[left] < ordered[right] })
	middle := len(ordered) / 2
	if len(ordered)%2 == 1 {
		return ordered[middle]
	}
	return (ordered[middle-1] + ordered[middle]) / 2
}
