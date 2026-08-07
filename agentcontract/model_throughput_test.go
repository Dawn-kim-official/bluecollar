package agentcontract

import (
	"testing"
	"time"
)

func recordCalls(observer *ThroughputObserver, modelName string, latencies ...time.Duration) {
	for _, latency := range latencies {
		observer.Record(modelName, latency, 200)
	}
}

func TestWithNothingMeasuredTheFloorStands(t *testing.T) {
	observer := NewThroughputObserver()

	if DurationForIterationCount(20, observer.Throughput("unseen/model")) != DurationForIterationCount(20, ModelThroughput{}) {
		t.Fatal("a model nobody has called yet gets the deadline the product promises, not a guess")
	}
}

func TestOneCallIsAlreadyBetterThanNoCall(t *testing.T) {
	observer := NewThroughputObserver()
	recordCalls(observer, "fast/model", time.Second)

	oneCall := DurationForIterationCount(20, observer.Throughput("fast/model"))

	if oneCall >= DurationForIterationCount(20, ModelThroughput{}) {
		t.Fatalf("one call is thin evidence but it is evidence, and waiting for eight of them leaves the first task on a stranger's clock: %s", oneCall)
	}
}

func TestTheEstimateIsTheMedianOfWhateverIsOnHand(t *testing.T) {
	observer := NewThroughputObserver()
	recordCalls(observer, "model", 1*time.Second, 9*time.Second, 2*time.Second)

	if observer.Throughput("model").CostPerCall != 2*time.Second {
		t.Fatalf("the median of one, nine and two seconds is two, and a mean would let one slow call move the deadline: %s", observer.Throughput("model").CostPerCall)
	}
}

func TestAnOutlierMattersLessAsEvidenceAccumulates(t *testing.T) {
	observer := NewThroughputObserver()
	recordCalls(observer, "model", time.Second)
	afterOne := observer.Throughput("model").CostPerCall
	for range 10 {
		recordCalls(observer, "model", time.Second)
	}
	recordCalls(observer, "model", 60*time.Second)
	afterMany := observer.Throughput("model").CostPerCall

	if afterMany != afterOne {
		t.Fatalf("one wild call among twelve should not move a median, got %s against %s", afterMany, afterOne)
	}
}

func TestTheWindowStopsGrowing(t *testing.T) {
	observer := NewThroughputObserver()
	for range throughputSampleCeiling + 50 {
		recordCalls(observer, "model", time.Second)
	}

	if observer.Throughput("model").SampleCount != throughputSampleCeiling {
		t.Fatalf("a model measured forever should answer for how it behaves now, got %d samples", observer.Throughput("model").SampleCount)
	}
}

func TestASlowModelStillOnlyGetsTheFloor(t *testing.T) {
	observer := NewThroughputObserver()
	recordCalls(observer, "slow/model", 60*time.Second, 60*time.Second, 60*time.Second)

	throughput := observer.Throughput("slow/model")

	if throughput.MeetsSupportedFloor() {
		t.Fatal("a minute a call is far below the floor and should be named as such")
	}
	if DurationForIterationCount(20, throughput) != DurationForIterationCount(20, ModelThroughput{}) {
		t.Fatal("and naming it does not earn it a longer clock")
	}
}

func TestTheDeadlineFollowsTheModelInUse(t *testing.T) {
	observer := NewThroughputObserver()
	recordCalls(observer, "fast/model", time.Second, time.Second, time.Second)
	recordCalls(observer, "slower/model", 5*time.Second, 5*time.Second, 5*time.Second)

	if observer.ThroughputOfModelInUse().CostPerCall != 5*time.Second {
		t.Fatal("after a switch the deadline belongs to the model now answering")
	}
}
