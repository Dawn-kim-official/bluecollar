package agentcontract

import (
	"testing"
	"time"
)

const testCostCeiling = time.Hour

func recordCalls(observer *ThroughputObserver, modelName string, latencies ...time.Duration) {
	for _, latency := range latencies {
		observer.Record(modelName, latency, 200)
	}
}

func TestWithNothingMeasuredTheUnmeasuredRateStands(t *testing.T) {
	observer := NewThroughputObserver()

	unseen := DurationForIterationCount(20, observer.Throughput("unseen/model"), testCostCeiling)

	if unseen != DurationForIterationCount(20, ModelThroughput{}, testCostCeiling) {
		t.Fatal("a model nobody has called yet has to be given some clock, and it is the one for a model we have not met")
	}
}

func TestOneCallIsAlreadyBetterThanNoCall(t *testing.T) {
	observer := NewThroughputObserver()
	recordCalls(observer, "fast/model", time.Second)

	oneCall := DurationForIterationCount(20, observer.Throughput("fast/model"), testCostCeiling)

	if oneCall >= DurationForIterationCount(20, ModelThroughput{}, testCostCeiling) {
		t.Fatalf("one call is thin evidence but it is evidence, and waiting for a quorum leaves the first task on a stranger's clock: %s", oneCall)
	}
}

func TestASlowModelIsGivenTheTimeItActuallyNeeds(t *testing.T) {
	observer := NewThroughputObserver()
	recordCalls(observer, "slow/model", 30*time.Second, 30*time.Second, 30*time.Second)

	slow := DurationForIterationCount(20, observer.Throughput("slow/model"), testCostCeiling)

	if slow <= DurationForIterationCount(20, ModelThroughput{}, testCostCeiling) {
		t.Fatalf("holding slow hardware to a faster machine's clock fails every task it is given, which is not a standard but a guarantee of failure: %s", slow)
	}
}

func TestTheCostCeilingIsWhatBoundsIt(t *testing.T) {
	observer := NewThroughputObserver()
	recordCalls(observer, "hung/model", time.Hour, time.Hour, time.Hour)

	bounded := DurationForIterationCount(20, observer.Throughput("hung/model"), 15*time.Minute)

	if bounded != 15*time.Minute {
		t.Fatalf("something has to stop a model that answers once an hour, and it is what the requester is willing to spend: %s", bounded)
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

	if observer.Throughput("model").CostPerCall != afterOne {
		t.Fatalf("one wild call among twelve should not move a median, got %s against %s", observer.Throughput("model").CostPerCall, afterOne)
	}
}

func TestTheWindowStopsGrowing(t *testing.T) {
	observer := NewThroughputObserver()
	for range throughputSampleCeiling + 50 {
		recordCalls(observer, "model", time.Second)
	}

	if observer.Throughput("model").SampleCount != throughputSampleCeiling {
		t.Fatalf("a model measured for months should answer for how it behaves now, got %d samples", observer.Throughput("model").SampleCount)
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

func TestAnImplausiblyFastMeasurementDoesNotCollapseTheDeadline(t *testing.T) {
	observer := NewThroughputObserver()
	recordCalls(observer, "instant/model", time.Millisecond, time.Millisecond, time.Millisecond)

	deadline := DurationForIterationCount(20, observer.Throughput("instant/model"), testCostCeiling)

	if deadline < 20*fastestPlausibleCostPerCall {
		t.Fatalf("a measurement near zero would hand a task a deadline near zero, and it would be blocked before its second call: %s", deadline)
	}
}
