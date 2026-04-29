package findings

import "testing"

func TestCoverageMapQuality(t *testing.T) {
	topics := CoverageMap()
	if len(topics) == 0 {
		t.Fatal("CoverageMap returned no topics")
	}
	seen := map[string]bool{}
	for _, topic := range topics {
		if topic.Topic == "" {
			t.Errorf("coverage topic with empty Topic: %+v", topic)
		}
		if seen[topic.Topic] {
			t.Errorf("duplicate coverage topic %q", topic.Topic)
		}
		seen[topic.Topic] = true
		if topic.Subsystem == "" {
			t.Errorf("coverage topic %q has empty Subsystem", topic.Topic)
		}
		if topic.Category == "" {
			t.Errorf("coverage topic %q has empty Category", topic.Topic)
		}
		switch topic.CoverageStatus {
		case CoverageCovered, CoveragePartial, CoverageDeferred, CoverageExcluded:
		default:
			t.Errorf("coverage topic %q has invalid status %q", topic.Topic, topic.CoverageStatus)
		}
		if topic.CoverageStatus != CoverageCovered && topic.Reason == "" {
			t.Errorf("coverage topic %q status %q needs a reason", topic.Topic, topic.CoverageStatus)
		}
	}
}
