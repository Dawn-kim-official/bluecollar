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
	answerWithoutToolsIterationCount        = 4
	answerWithoutToolsToolCallCount         = 1
)

func escalatedFrom(firstTierBudget int, doublings int) int {
	return firstTierBudget << doublings
}

var taskLevelProfiles = []TaskLevelProfile{
	{TaskLevel: TaskLevelXLow, Duration: 3 * time.Minute, MaxIterationCount: answerWithoutToolsIterationCount, MaxToolCallCount: answerWithoutToolsToolCallCount},
	{TaskLevel: TaskLevelLow, Duration: 10 * time.Minute, MaxIterationCount: escalatedFrom(measuredSuccessfulIterationPercentile95, 0), MaxToolCallCount: escalatedFrom(measuredSuccessfulToolCallPercentile95, 0)},
	{TaskLevel: TaskLevelMedium, Duration: 20 * time.Minute, MaxIterationCount: escalatedFrom(measuredSuccessfulIterationPercentile95, 1), MaxToolCallCount: escalatedFrom(measuredSuccessfulToolCallPercentile95, 1)},
	{TaskLevel: TaskLevelHigh, Duration: 40 * time.Minute, MaxIterationCount: escalatedFrom(measuredSuccessfulIterationPercentile95, 2), MaxToolCallCount: escalatedFrom(measuredSuccessfulToolCallPercentile95, 2)},
	{TaskLevel: TaskLevelXHigh, Duration: time.Hour, MaxIterationCount: escalatedFrom(measuredSuccessfulIterationPercentile95, 3), MaxToolCallCount: escalatedFrom(measuredSuccessfulToolCallPercentile95, 3)},
	{TaskLevel: TaskLevelMax, Duration: time.Hour, MaxIterationCount: escalatedFrom(measuredSuccessfulIterationPercentile95, 4), MaxToolCallCount: escalatedFrom(measuredSuccessfulToolCallPercentile95, 4)},
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
