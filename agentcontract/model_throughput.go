package agentcontract

import (
	"sync"
	"time"
)

type ModelThroughput struct {
	CostPerCall           time.Duration
	OutputTokensPerSecond float64
	fittedAtSampleCount   int
}

func (throughput ModelThroughput) costOfOneCall() time.Duration {
	generating := time.Duration(float64(measuredOutputTokensPerModelCall) / throughput.OutputTokensPerSecond * float64(time.Second))
	return throughput.CostPerCall + generating
}

func supportedFloorThroughput() ModelThroughput {
	return ModelThroughput{CostPerCall: localCostPerModelCall, OutputTokensPerSecond: supportedFloorOutputTokensPerSecond}
}

func (throughput ModelThroughput) MeetsSupportedFloor() bool {
	return throughput.OutputTokensPerSecond <= 0 || throughput.OutputTokensPerSecond >= supportedFloorOutputTokensPerSecond
}

func DurationForIterationCount(iterationCount int, throughput ModelThroughput) time.Duration {
	floor := time.Duration(iterationCount) * supportedFloorThroughput().costOfOneCall() * durationMargin
	if throughput.OutputTokensPerSecond <= 0 {
		return floor
	}
	observed := time.Duration(iterationCount) * throughput.costOfOneCall() * durationMargin
	return min(observed, floor)
}

type modelCallSample struct {
	latency          time.Duration
	completionTokens int64
}

type ThroughputObserver struct {
	mutex           sync.Mutex
	samplesByModel  map[string][]modelCallSample
	fittedByModel   map[string]ModelThroughput
	lastRecordedFor string
}

const (
	throughputSampleCeiling  = 200
	throughputSampleMinimum  = 8
	throughputRefitThreshold = 2
)

func NewThroughputObserver() *ThroughputObserver {
	return &ThroughputObserver{
		samplesByModel: map[string][]modelCallSample{},
		fittedByModel:  map[string]ModelThroughput{},
	}
}

func (observer *ThroughputObserver) Record(modelName string, latency time.Duration, completionTokens int64) {
	if observer == nil || modelName == "" || latency <= 0 || completionTokens <= 0 {
		return
	}
	observer.mutex.Lock()
	defer observer.mutex.Unlock()
	samples := append(observer.samplesByModel[modelName], modelCallSample{latency: latency, completionTokens: completionTokens})
	if len(samples) > throughputSampleCeiling {
		samples = samples[len(samples)-throughputSampleCeiling:]
	}
	observer.samplesByModel[modelName] = samples
	observer.lastRecordedFor = modelName
	if observer.shouldRefit(modelName, len(samples)) {
		delete(observer.fittedByModel, modelName)
	}
}

func (observer *ThroughputObserver) shouldRefit(modelName string, sampleCount int) bool {
	fitted, wasFitted := observer.fittedByModel[modelName]
	if !wasFitted {
		return false
	}
	return sampleCount >= fitted.fittedAtSampleCount*throughputRefitThreshold
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
	if fitted, wasFitted := observer.fittedByModel[modelName]; wasFitted {
		return fitted
	}
	samples := observer.samplesByModel[modelName]
	if len(samples) < throughputSampleMinimum {
		return ModelThroughput{}
	}
	fitted := fitModelThroughput(samples)
	observer.fittedByModel[modelName] = fitted
	return fitted
}

func fitModelThroughput(samples []modelCallSample) ModelThroughput {
	callCount := float64(len(samples))
	var tokenSum, latencySum, tokenLatencySum, tokenSquareSum float64
	for _, sample := range samples {
		tokens := float64(sample.completionTokens)
		seconds := sample.latency.Seconds()
		tokenSum += tokens
		latencySum += seconds
		tokenLatencySum += tokens * seconds
		tokenSquareSum += tokens * tokens
	}
	denominator := callCount*tokenSquareSum - tokenSum*tokenSum
	if denominator == 0 {
		return ModelThroughput{}
	}
	secondsPerToken := (callCount*tokenLatencySum - tokenSum*latencySum) / denominator
	fixedSeconds := (latencySum - secondsPerToken*tokenSum) / callCount
	if secondsPerToken <= 0 || fixedSeconds < 0 {
		return ModelThroughput{}
	}
	return ModelThroughput{
		CostPerCall:           time.Duration(fixedSeconds * float64(time.Second)),
		OutputTokensPerSecond: 1 / secondsPerToken,
		fittedAtSampleCount:   len(samples),
	}
}
