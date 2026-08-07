package agentcontract

import (
	"testing"
	"time"
)

func recordCalls(observer *ThroughputObserver, modelName string, callCount int, fixed time.Duration, tokensPerSecond float64, completionTokens int64) {
	for callIndex := 0; callIndex < callCount; callIndex++ {
		tokens := completionTokens + int64(callIndex*10)
		latency := fixed + time.Duration(float64(tokens)/tokensPerSecond*float64(time.Second))
		observer.Record(modelName, latency, tokens)
	}
}

func TestAFastModelEarnsATighterDeadlineThanTheFloor(t *testing.T) {
	observer := NewThroughputObserver()
	recordCalls(observer, "fast/model", 20, 900*time.Millisecond, 340, 200)

	fast := DurationForIterationCount(20, observer.Throughput("fast/model"))
	floor := DurationForIterationCount(20, ModelThroughput{})

	if fast >= floor {
		t.Fatalf("a model measured seventeen times faster than the supported floor should not be held to the floor's clock, got %s against %s", fast, floor)
	}
}

func TestAModelNobodyHasMeasuredKeepsTheFloor(t *testing.T) {
	observer := NewThroughputObserver()
	recordCalls(observer, "new/model", throughputSampleMinimum-1, time.Second, 300, 200)

	unmeasured := DurationForIterationCount(20, observer.Throughput("new/model"))

	if unmeasured != DurationForIterationCount(20, ModelThroughput{}) {
		t.Fatalf("a handful of calls is not a measurement, and guessing from it can only cut a task short: %s", unmeasured)
	}
}

func TestASlowModelIsNeverGivenLessThanTheFloor(t *testing.T) {
	observer := NewThroughputObserver()
	recordCalls(observer, "slow/model", 20, 2*time.Second, 5, 200)

	slow := DurationForIterationCount(20, observer.Throughput("slow/model"))

	if slow != DurationForIterationCount(20, ModelThroughput{}) {
		t.Fatalf("the floor is the deadline the product promises to support, and a slower model does not get to shorten it: %s", slow)
	}
}

func TestTheFitIsNotRedoneWhileTheSettingHoldsStill(t *testing.T) {
	observer := NewThroughputObserver()
	recordCalls(observer, "steady/model", 20, 900*time.Millisecond, 340, 200)

	firstFit := observer.Throughput("steady/model")
	observer.Record("steady/model", 30*time.Second, 100)
	secondFit := observer.Throughput("steady/model")

	if secondFit != firstFit {
		t.Fatal("one more call is not a new setting, and refitting on every call spends work to chase noise")
	}
}

func TestChangingTheModelChangesTheAnswer(t *testing.T) {
	observer := NewThroughputObserver()
	recordCalls(observer, "fast/model", 20, 900*time.Millisecond, 340, 200)
	recordCalls(observer, "slow/model", 20, 2*time.Second, 30, 200)

	if observer.ThroughputOfModelInUse().OutputTokensPerSecond >= 100 {
		t.Fatal("the deadline follows the model actually answering, and after a switch that is the new one")
	}
}
