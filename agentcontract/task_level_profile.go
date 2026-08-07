package agentcontract

import (
	"time"
)

type TaskLevelProfile struct {
	TaskLevel         TaskLevel
	Duration          time.Duration
	MaxIterationCount int
	MaxToolCallCount  int
}

const (
	measuredSuccessfulIterationPercentile95 = 20
	measuredSuccessfulToolCallPercentile95  = 13
	measuredSuccessfulDurationPercentile95  = 4 * time.Minute
	firstTierDurationMargin                 = 2
	answerWithoutToolsIterationCount        = 4
	answerWithoutToolsToolCallCount         = 1
	answerWithoutToolsDuration              = 3 * time.Minute
)

func escalatedFrom(firstTierBudget int, doublings int) int {
	return firstTierBudget << doublings
}

func escalatedDuration(doublings int) time.Duration {
	return measuredSuccessfulDurationPercentile95 * firstTierDurationMargin << doublings
}

var taskLevelProfiles = []TaskLevelProfile{
	{TaskLevel: TaskLevelXLow, Duration: answerWithoutToolsDuration, MaxIterationCount: answerWithoutToolsIterationCount, MaxToolCallCount: answerWithoutToolsToolCallCount},
	{TaskLevel: TaskLevelLow, Duration: escalatedDuration(0), MaxIterationCount: escalatedFrom(measuredSuccessfulIterationPercentile95, 0), MaxToolCallCount: escalatedFrom(measuredSuccessfulToolCallPercentile95, 0)},
	{TaskLevel: TaskLevelMedium, Duration: escalatedDuration(1), MaxIterationCount: escalatedFrom(measuredSuccessfulIterationPercentile95, 1), MaxToolCallCount: escalatedFrom(measuredSuccessfulToolCallPercentile95, 1)},
	{TaskLevel: TaskLevelHigh, Duration: escalatedDuration(2), MaxIterationCount: escalatedFrom(measuredSuccessfulIterationPercentile95, 2), MaxToolCallCount: escalatedFrom(measuredSuccessfulToolCallPercentile95, 2)},
	{TaskLevel: TaskLevelXHigh, Duration: escalatedDuration(3), MaxIterationCount: escalatedFrom(measuredSuccessfulIterationPercentile95, 3), MaxToolCallCount: escalatedFrom(measuredSuccessfulToolCallPercentile95, 3)},
	{TaskLevel: TaskLevelMax, Duration: escalatedDuration(4), MaxIterationCount: escalatedFrom(measuredSuccessfulIterationPercentile95, 4), MaxToolCallCount: escalatedFrom(measuredSuccessfulToolCallPercentile95, 4)},
}

func TaskLevelProfileForLevel(taskLevel TaskLevel) TaskLevelProfile {
	normalizedTaskLevel := NormalizeTaskLevel(string(taskLevel))
	if normalizedTaskLevel == "" {
		normalizedTaskLevel = TaskLevelLow
	}
	for _, taskLevelProfile := range taskLevelProfiles {
		if taskLevelProfile.TaskLevel == normalizedTaskLevel {
			return taskLevelProfile
		}
	}
	return taskLevelProfiles[1]
}

func NextTaskLevel(taskLevel TaskLevel) (TaskLevel, bool) {
	currentRank := TaskLevelRank(TaskLevelProfileForLevel(taskLevel).TaskLevel)
	if currentRank < 0 || currentRank+1 >= len(taskLevelProfiles) {
		return "", false
	}
	return taskLevelProfiles[currentRank+1].TaskLevel, true
}

func TaskLevelWantsProgressCheckpoints(taskLevel TaskLevel) bool {
	return TaskLevelRank(taskLevel) >= TaskLevelRank(TaskLevelMedium)
}

func TaskLevelWantsSingleFinalReply(taskLevel TaskLevel) bool {
	return NormalizeTaskLevel(string(taskLevel)) == TaskLevelXLow
}

func TaskLevelRequiresPlan(taskLevel TaskLevel) bool {
	return TaskLevelRank(taskLevel) >= TaskLevelRank(TaskLevelMedium)
}
